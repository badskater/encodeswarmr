package api

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/db"
)

// enrollmentTokenResponse is the public representation returned in list/get
// operations. It omits the raw token value for security.
type enrollmentTokenResponse struct {
	ID        string  `json:"id"`
	UsedBy    *string `json:"used_by,omitempty"`
	UsedAt    *string `json:"used_at,omitempty"`
	ExpiresAt string  `json:"expires_at"`
	CreatedAt string  `json:"created_at"`
}

// enrollmentTokenCreateResponse includes the token value, shown only once at
// creation time.
type enrollmentTokenCreateResponse struct {
	enrollmentTokenResponse
	Token string `json:"token"`
}

func toEnrollmentTokenResponse(t *db.EnrollmentToken) enrollmentTokenResponse {
	resp := enrollmentTokenResponse{
		ID:        t.ID,
		UsedBy:    t.UsedBy,
		ExpiresAt: t.ExpiresAt.Format(time.RFC3339),
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
	if t.UsedAt != nil {
		s := t.UsedAt.Format(time.RFC3339)
		resp.UsedAt = &s
	}
	return resp
}

// handleCreateEnrollmentToken creates a new one-time enrollment token.
// POST /api/v1/agent-tokens
func (s *Server) handleCreateEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpiresIn string `json:"expires_in"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	expiry := 24 * time.Hour
	if req.ExpiresIn != "" {
		d, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "invalid expires_in duration")
			return
		}
		expiry = d
	}

	// Generate 32-byte random hex token.
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		s.logger.Error("generate enrollment token", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	tokenStr := hex.EncodeToString(tokenBytes)

	// Determine created_by from the authenticated user context.
	createdBy := "system"
	if claims, ok := auth.FromContext(r.Context()); ok {
		createdBy = claims.UserID
	}

	tok, err := s.store.CreateEnrollmentToken(r.Context(), db.CreateEnrollmentTokenParams{
		Token:     tokenStr,
		CreatedBy: createdBy,
		ExpiresAt: time.Now().Add(expiry),
	})
	if err != nil {
		s.logger.Error("create enrollment token", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Emit audit entry — best-effort, never fail the request.
	auditParams := db.CreateAuditEntryParams{
		Action:     "token.create",
		Resource:   "enrollment_token",
		ResourceID: tok.ID,
		IPAddress:  r.RemoteAddr,
	}
	if claims, ok := auth.FromContext(r.Context()); ok {
		auditParams.UserID = &claims.UserID
		auditParams.Username = claims.Username
	}
	if err := s.store.CreateAuditEntry(r.Context(), auditParams); err != nil {
		s.logger.Warn("audit log: create enrollment token", "err", err, "token_id", tok.ID)
	}

	resp := enrollmentTokenCreateResponse{
		enrollmentTokenResponse: toEnrollmentTokenResponse(tok),
		Token:                   tok.Token,
	}
	writeJSON(w, r, http.StatusCreated, resp)
}

// handleListEnrollmentTokens lists all enrollment tokens (without token values).
// GET /api/v1/agent-tokens
func (s *Server) handleListEnrollmentTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.store.ListEnrollmentTokens(r.Context())
	if err != nil {
		s.logger.Error("list enrollment tokens", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	out := make([]enrollmentTokenResponse, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, toEnrollmentTokenResponse(t))
	}
	writeJSON(w, r, http.StatusOK, out)
}

// handleDeleteEnrollmentToken deletes an enrollment token by ID.
// DELETE /api/v1/agent-tokens/{id}
func (s *Server) handleDeleteEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.DeleteEnrollmentToken(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "enrollment token not found")
		return
	}
	if err != nil {
		s.logger.Error("delete enrollment token", "err", err, "token_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleAgentEnroll processes an agent enrollment request using a one-time token.
// POST /api/v1/agent/enroll
func (s *Server) handleAgentEnroll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token  string `json:"token"`
		CSRPEM string `json:"csr_pem"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Token == "" || req.CSRPEM == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "token and csr_pem are required")
		return
	}

	// Validate the enrollment token.
	tok, err := s.store.GetEnrollmentToken(r.Context(), req.Token)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusUnauthorized, "Unauthorized", "invalid, expired, or already used token")
		return
	}
	if err != nil {
		s.logger.Error("get enrollment token", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Parse and verify the CSR.
	csrBlock, _ := pem.Decode([]byte(req.CSRPEM))
	if csrBlock == nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid PEM-encoded CSR")
		return
	}
	csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "failed to parse CSR: "+err.Error())
		return
	}
	if err := csr.CheckSignature(); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "CSR signature verification failed")
		return
	}

	// Load CA cert and key for signing.
	if s.cfg.TLS.CertFile == "" {
		writeProblem(w, r, http.StatusServiceUnavailable, "Service Unavailable", "TLS not configured")
		return
	}
	caCertPEM, err := os.ReadFile(s.cfg.TLS.CertFile)
	if err != nil {
		s.logger.Error("read CA cert", "err", err, "path", s.cfg.TLS.CertFile)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	caKeyPEM, err := os.ReadFile(s.cfg.TLS.KeyFile)
	if err != nil {
		s.logger.Error("read CA key", "err", err, "path", s.cfg.TLS.KeyFile)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	certPEM, err := signCSR(caCertPEM, caKeyPEM, csr)
	if err != nil {
		s.logger.Error("sign CSR", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Consume the token. Use the token ID as a placeholder agent ID since
	// the agent record may not exist yet.
	if err := s.store.ConsumeEnrollmentToken(r.Context(), db.ConsumeEnrollmentTokenParams{
		Token:   tok.Token,
		AgentID: tok.ID,
	}); err != nil {
		s.logger.Error("consume enrollment token", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]string{
		"cert_pem": string(certPEM),
	})
}

// signCSR signs a certificate request using the provided CA certificate and key.
// It returns a PEM-encoded certificate valid for 1 year.
func signCSR(caCertPEM, caKeyPEM []byte, csr *x509.CertificateRequest) ([]byte, error) {
	// Parse CA certificate.
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil {
		return nil, errors.New("failed to decode CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return nil, errors.New("failed to parse CA certificate: " + err.Error())
	}

	// Parse CA private key.
	keyBlock, _ := pem.Decode(caKeyPEM)
	if keyBlock == nil {
		return nil, errors.New("failed to decode CA key PEM")
	}
	caKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		// Fallback: try PKCS1 and EC key formats.
		caKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			caKey, err = x509.ParseECPrivateKey(keyBlock.Bytes)
			if err != nil {
				return nil, errors.New("failed to parse CA private key")
			}
		}
	}

	// Generate a random serial number.
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, errors.New("failed to generate serial number: " + err.Error())
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: csr.Subject.CommonName,
		},
		NotBefore: now.Add(-5 * time.Minute), // small clock-skew tolerance
		NotAfter:  now.Add(365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, csr.PublicKey, caKey)
	if err != nil {
		return nil, errors.New("failed to create certificate: " + err.Error())
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	return certPEM, nil
}
