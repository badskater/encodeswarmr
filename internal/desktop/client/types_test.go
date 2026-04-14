package client

import (
	"encoding/json"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// ptr returns a pointer to v for nullable field tests.
func ptr[T any](v T) *T { return &v }

// ---------------------------------------------------------------------------
// Status constants
// ---------------------------------------------------------------------------

func TestJobStatusConstants(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"JobQueued", JobQueued, "queued"},
		{"JobWaiting", JobWaiting, "waiting"},
		{"JobAssigned", JobAssigned, "assigned"},
		{"JobRunning", JobRunning, "running"},
		{"JobCompleted", JobCompleted, "completed"},
		{"JobFailed", JobFailed, "failed"},
		{"JobCancelled", JobCancelled, "cancelled"},
	}
	for _, tc := range cases {
		if tc.value != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.value, tc.want)
		}
	}
}

func TestTaskStatusConstants(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"TaskPending", TaskPending, "pending"},
		{"TaskAssigned", TaskAssigned, "assigned"},
		{"TaskRunning", TaskRunning, "running"},
		{"TaskCompleted", TaskCompleted, "completed"},
		{"TaskFailed", TaskFailed, "failed"},
	}
	for _, tc := range cases {
		if tc.value != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.value, tc.want)
		}
	}
}

func TestAgentStatusConstants(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"AgentIdle", AgentIdle, "idle"},
		{"AgentRunning", AgentRunning, "running"},
		{"AgentOffline", AgentOffline, "offline"},
		{"AgentDraining", AgentDraining, "draining"},
		{"AgentPendingApproval", AgentPendingApproval, "pending_approval"},
	}
	for _, tc := range cases {
		if tc.value != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.value, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Job marshaling / unmarshaling
// ---------------------------------------------------------------------------

func TestJob_MarshalUnmarshal_AllFields(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	eta := 120.5
	etaHuman := "2 minutes"
	chainGroup := "cg-1"
	dependsOn := "j-prev"

	job := Job{
		ID:             "job-abc",
		SourceID:       "src-1",
		SourcePath:     "/mnt/media/movie.mkv",
		JobType:        "encode",
		Status:         JobRunning,
		Priority:       5,
		TasksTotal:     10,
		TasksCompleted: 3,
		TasksFailed:    1,
		TasksPending:   4,
		TasksRunning:   2,
		DependsOn:      &dependsOn,
		ChainGroup:     &chainGroup,
		TargetTags:     []string{"gpu", "fast"},
		AudioConfig: &AudioConfig{
			Codec:      "aac",
			Bitrate:    ptr("192k"),
			Channels:   ptr(2),
			SampleRate: ptr(48000),
		},
		ETASeconds: &eta,
		ETAHuman:   &etaHuman,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Job
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.ID != job.ID {
		t.Errorf("ID = %q, want %q", got.ID, job.ID)
	}
	if got.Status != job.Status {
		t.Errorf("Status = %q, want %q", got.Status, job.Status)
	}
	if got.Priority != job.Priority {
		t.Errorf("Priority = %d, want %d", got.Priority, job.Priority)
	}
	if got.TasksTotal != job.TasksTotal {
		t.Errorf("TasksTotal = %d, want %d", got.TasksTotal, job.TasksTotal)
	}
	if got.DependsOn == nil || *got.DependsOn != dependsOn {
		t.Errorf("DependsOn = %v, want %q", got.DependsOn, dependsOn)
	}
	if got.ChainGroup == nil || *got.ChainGroup != chainGroup {
		t.Errorf("ChainGroup = %v, want %q", got.ChainGroup, chainGroup)
	}
	if len(got.TargetTags) != 2 || got.TargetTags[0] != "gpu" {
		t.Errorf("TargetTags = %v, want [gpu fast]", got.TargetTags)
	}
	if got.AudioConfig == nil {
		t.Fatal("AudioConfig is nil")
	}
	if got.AudioConfig.Codec != "aac" {
		t.Errorf("AudioConfig.Codec = %q, want aac", got.AudioConfig.Codec)
	}
	if got.AudioConfig.Bitrate == nil || *got.AudioConfig.Bitrate != "192k" {
		t.Errorf("AudioConfig.Bitrate = %v, want 192k", got.AudioConfig.Bitrate)
	}
	if got.ETASeconds == nil || *got.ETASeconds != eta {
		t.Errorf("ETASeconds = %v, want %v", got.ETASeconds, eta)
	}
	if got.ETAHuman == nil || *got.ETAHuman != etaHuman {
		t.Errorf("ETAHuman = %v, want %q", got.ETAHuman, etaHuman)
	}
}

func TestJob_NullableFields_Absent(t *testing.T) {
	// When nullable pointer fields are absent from JSON they must unmarshal as nil.
	raw := `{
		"id": "j-null",
		"source_id": "s1",
		"source_path": "/a/b.mkv",
		"job_type": "encode",
		"status": "queued",
		"priority": 0,
		"tasks_total": 0,
		"tasks_completed": 0,
		"tasks_failed": 0,
		"tasks_pending": 0,
		"tasks_running": 0,
		"created_at": "2024-01-01T00:00:00Z",
		"updated_at": "2024-01-01T00:00:00Z"
	}`
	var job Job
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if job.DependsOn != nil {
		t.Errorf("DependsOn = %v, want nil", job.DependsOn)
	}
	if job.ChainGroup != nil {
		t.Errorf("ChainGroup = %v, want nil", job.ChainGroup)
	}
	if job.AudioConfig != nil {
		t.Errorf("AudioConfig = %v, want nil", job.AudioConfig)
	}
	if job.ETASeconds != nil {
		t.Errorf("ETASeconds = %v, want nil", job.ETASeconds)
	}
	if job.ETAHuman != nil {
		t.Errorf("ETAHuman = %v, want nil", job.ETAHuman)
	}
}

func TestJob_NilPointerFields_MarshalAsNull(t *testing.T) {
	// Pointer fields without omitempty must marshal as JSON null, not be absent.
	// This matches the server contract where the client always receives the key.
	job := Job{
		ID:        "j-null-marshal",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal map: %v", err)
	}
	// eta_seconds and eta_human have no omitempty tag; they appear as null.
	for _, key := range []string{"eta_seconds", "eta_human", "depends_on", "chain_group"} {
		val, ok := raw[key]
		if !ok {
			t.Errorf("%q should be present in JSON (as null), but was absent", key)
			continue
		}
		if string(val) != "null" {
			t.Errorf("%q = %s, want null", key, val)
		}
	}
}

// ---------------------------------------------------------------------------
// Task marshaling / unmarshaling
// ---------------------------------------------------------------------------

func TestTask_MarshalUnmarshal(t *testing.T) {
	now := time.Date(2024, 3, 10, 9, 0, 0, 0, time.UTC)
	agentID := "agent-1"
	exitCode := 0
	errMsg := "some error"
	frames := int64(24000)
	fps := 23.976
	outSize := int64(1073741824)
	startedAt := now
	completedAt := now.Add(10 * time.Minute)

	task := Task{
		ID:            "t-1",
		JobID:         "j-1",
		AgentID:       &agentID,
		Status:        TaskCompleted,
		ChunkIndex:    3,
		ExitCode:      &exitCode,
		ErrorMsg:      &errMsg,
		FramesEncoded: &frames,
		AvgFPS:        &fps,
		OutputSize:    &outSize,
		RetryCount:    1,
		StartedAt:     &startedAt,
		CompletedAt:   &completedAt,
		CreatedAt:     now,
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Task
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.ID != task.ID {
		t.Errorf("ID = %q, want %q", got.ID, task.ID)
	}
	if got.AgentID == nil || *got.AgentID != agentID {
		t.Errorf("AgentID = %v, want %q", got.AgentID, agentID)
	}
	if got.Status != TaskCompleted {
		t.Errorf("Status = %q, want %q", got.Status, TaskCompleted)
	}
	if got.ChunkIndex != 3 {
		t.Errorf("ChunkIndex = %d, want 3", got.ChunkIndex)
	}
	if got.ExitCode == nil || *got.ExitCode != exitCode {
		t.Errorf("ExitCode = %v, want %d", got.ExitCode, exitCode)
	}
	if got.FramesEncoded == nil || *got.FramesEncoded != frames {
		t.Errorf("FramesEncoded = %v, want %d", got.FramesEncoded, frames)
	}
	if got.AvgFPS == nil || *got.AvgFPS != fps {
		t.Errorf("AvgFPS = %v, want %v", got.AvgFPS, fps)
	}
	if got.OutputSize == nil || *got.OutputSize != outSize {
		t.Errorf("OutputSize = %v, want %d", got.OutputSize, outSize)
	}
	if got.RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", got.RetryCount)
	}
}

func TestTask_NullableFields_Absent(t *testing.T) {
	raw := `{
		"id": "t-nil",
		"job_id": "j-nil",
		"status": "pending",
		"chunk_index": 0,
		"created_at": "2024-01-01T00:00:00Z"
	}`
	var task Task
	if err := json.Unmarshal([]byte(raw), &task); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if task.AgentID != nil {
		t.Errorf("AgentID = %v, want nil", task.AgentID)
	}
	if task.ExitCode != nil {
		t.Errorf("ExitCode = %v, want nil", task.ExitCode)
	}
	if task.ErrorMsg != nil {
		t.Errorf("ErrorMsg = %v, want nil", task.ErrorMsg)
	}
	if task.FramesEncoded != nil {
		t.Errorf("FramesEncoded = %v, want nil", task.FramesEncoded)
	}
	if task.AvgFPS != nil {
		t.Errorf("AvgFPS = %v, want nil", task.AvgFPS)
	}
	if task.OutputSize != nil {
		t.Errorf("OutputSize = %v, want nil", task.OutputSize)
	}
	if task.StartedAt != nil {
		t.Errorf("StartedAt = %v, want nil", task.StartedAt)
	}
	if task.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil", task.CompletedAt)
	}
}

// ---------------------------------------------------------------------------
// Agent marshaling / unmarshaling
// ---------------------------------------------------------------------------

func TestAgent_MarshalUnmarshal(t *testing.T) {
	now := time.Date(2024, 4, 20, 8, 0, 0, 0, time.UTC)
	heartbeat := now
	gpuVendor := "NVIDIA"
	gpuModel := "RTX 4090"

	agent := Agent{
		ID:            "ag-1",
		Name:          "encoder-01",
		Hostname:      "WIN-ENC01",
		IPAddress:     "192.168.1.100",
		Status:        AgentIdle,
		Tags:          []string{"gpu", "nvenc"},
		AgentVersion:  "1.2.3",
		OSVersion:     "Windows Server 2022",
		CPUCount:      16,
		RAMMIB:        32768,
		GPUVendor:     &gpuVendor,
		GPUModel:      &gpuModel,
		GPUEnabled:    true,
		NVENC:         true,
		QSV:           false,
		AMF:           false,
		VNCPort:       5900,
		LastHeartbeat: &heartbeat,
		CreatedAt:     now,
		UpdateChannel: "stable",
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Agent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.ID != agent.ID {
		t.Errorf("ID = %q, want %q", got.ID, agent.ID)
	}
	if got.Status != AgentIdle {
		t.Errorf("Status = %q, want %q", got.Status, AgentIdle)
	}
	if len(got.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(got.Tags))
	}
	if got.CPUCount != 16 {
		t.Errorf("CPUCount = %d, want 16", got.CPUCount)
	}
	if got.RAMMIB != 32768 {
		t.Errorf("RAMMIB = %d, want 32768", got.RAMMIB)
	}
	if got.GPUVendor == nil || *got.GPUVendor != gpuVendor {
		t.Errorf("GPUVendor = %v, want %q", got.GPUVendor, gpuVendor)
	}
	if got.GPUModel == nil || *got.GPUModel != gpuModel {
		t.Errorf("GPUModel = %v, want %q", got.GPUModel, gpuModel)
	}
	if !got.GPUEnabled {
		t.Error("GPUEnabled = false, want true")
	}
	if !got.NVENC {
		t.Error("NVENC = false, want true")
	}
	if got.QSV {
		t.Error("QSV = true, want false")
	}
	if got.UpdateChannel != "stable" {
		t.Errorf("UpdateChannel = %q, want %q", got.UpdateChannel, "stable")
	}
}

func TestAgent_NullableFields_Absent(t *testing.T) {
	raw := `{
		"id": "ag-nil",
		"name": "enc-02",
		"hostname": "WIN-ENC02",
		"ip_address": "10.0.0.2",
		"status": "offline",
		"agent_version": "1.0.0",
		"os_version": "Windows Server 2019",
		"cpu_count": 8,
		"ram_mib": 16384,
		"gpu_enabled": false,
		"nvenc": false,
		"qsv": false,
		"amf": false,
		"vnc_port": 5901,
		"created_at": "2024-01-01T00:00:00Z",
		"update_channel": "stable"
	}`
	var agent Agent
	if err := json.Unmarshal([]byte(raw), &agent); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if agent.GPUVendor != nil {
		t.Errorf("GPUVendor = %v, want nil", agent.GPUVendor)
	}
	if agent.GPUModel != nil {
		t.Errorf("GPUModel = %v, want nil", agent.GPUModel)
	}
	if agent.LastHeartbeat != nil {
		t.Errorf("LastHeartbeat = %v, want nil", agent.LastHeartbeat)
	}
}

// ---------------------------------------------------------------------------
// Source marshaling / unmarshaling
// ---------------------------------------------------------------------------

func TestSource_MarshalUnmarshal(t *testing.T) {
	now := time.Date(2024, 7, 4, 0, 0, 0, 0, time.UTC)
	duration := 5400.5
	vmaf := 95.3
	cloudURI := "s3://bucket/path/movie.mkv"

	source := Source{
		ID:          "src-1",
		Path:        "/mnt/media/movie.mkv",
		Filename:    "movie.mkv",
		SizeBytes:   10737418240,
		DurationSec: &duration,
		State:       "ready",
		VMafScore:   &vmaf,
		CloudURI:    &cloudURI,
		HDRType:     "HDR10",
		DVProfile:   7,
		Thumbnails:  []string{"thumb1.jpg", "thumb2.jpg"},
		CreatedAt:   now,
	}

	data, err := json.Marshal(source)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Source
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.ID != source.ID {
		t.Errorf("ID = %q, want %q", got.ID, source.ID)
	}
	if got.Filename != source.Filename {
		t.Errorf("Filename = %q, want %q", got.Filename, source.Filename)
	}
	if got.SizeBytes != source.SizeBytes {
		t.Errorf("SizeBytes = %d, want %d", got.SizeBytes, source.SizeBytes)
	}
	if got.DurationSec == nil || *got.DurationSec != duration {
		t.Errorf("DurationSec = %v, want %v", got.DurationSec, duration)
	}
	if got.VMafScore == nil || *got.VMafScore != vmaf {
		t.Errorf("VMafScore = %v, want %v", got.VMafScore, vmaf)
	}
	if got.CloudURI == nil || *got.CloudURI != cloudURI {
		t.Errorf("CloudURI = %v, want %q", got.CloudURI, cloudURI)
	}
	if got.HDRType != "HDR10" {
		t.Errorf("HDRType = %q, want HDR10", got.HDRType)
	}
	if got.DVProfile != 7 {
		t.Errorf("DVProfile = %d, want 7", got.DVProfile)
	}
	if len(got.Thumbnails) != 2 {
		t.Errorf("len(Thumbnails) = %d, want 2", len(got.Thumbnails))
	}
}

func TestSource_NullableFields_Absent(t *testing.T) {
	raw := `{
		"id": "src-nil",
		"path": "/mnt/video/clip.mp4",
		"filename": "clip.mp4",
		"size_bytes": 1048576,
		"state": "pending",
		"hdr_type": "SDR",
		"dv_profile": 0,
		"thumbnails": [],
		"created_at": "2024-01-01T00:00:00Z"
	}`
	var source Source
	if err := json.Unmarshal([]byte(raw), &source); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if source.DurationSec != nil {
		t.Errorf("DurationSec = %v, want nil", source.DurationSec)
	}
	if source.VMafScore != nil {
		t.Errorf("VMafScore = %v, want nil", source.VMafScore)
	}
	if source.CloudURI != nil {
		t.Errorf("CloudURI = %v, want nil", source.CloudURI)
	}
}

// ---------------------------------------------------------------------------
// AudioConfig nullable fields
// ---------------------------------------------------------------------------

func TestAudioConfig_NullableFields(t *testing.T) {
	raw := `{"codec":"flac"}`
	var ac AudioConfig
	if err := json.Unmarshal([]byte(raw), &ac); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if ac.Codec != "flac" {
		t.Errorf("Codec = %q, want flac", ac.Codec)
	}
	if ac.Bitrate != nil {
		t.Errorf("Bitrate = %v, want nil", ac.Bitrate)
	}
	if ac.Channels != nil {
		t.Errorf("Channels = %v, want nil", ac.Channels)
	}
	if ac.SampleRate != nil {
		t.Errorf("SampleRate = %v, want nil", ac.SampleRate)
	}
}

func TestAudioConfig_WithAllFields(t *testing.T) {
	bitrate := "320k"
	channels := 6
	sampleRate := 96000

	ac := AudioConfig{
		Codec:      "opus",
		Bitrate:    &bitrate,
		Channels:   &channels,
		SampleRate: &sampleRate,
	}

	data, err := json.Marshal(ac)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got AudioConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Codec != "opus" {
		t.Errorf("Codec = %q, want opus", got.Codec)
	}
	if got.Bitrate == nil || *got.Bitrate != bitrate {
		t.Errorf("Bitrate = %v, want %q", got.Bitrate, bitrate)
	}
	if got.Channels == nil || *got.Channels != channels {
		t.Errorf("Channels = %v, want %d", got.Channels, channels)
	}
	if got.SampleRate == nil || *got.SampleRate != sampleRate {
		t.Errorf("SampleRate = %v, want %d", got.SampleRate, sampleRate)
	}
}

// ---------------------------------------------------------------------------
// User marshaling
// ---------------------------------------------------------------------------

func TestUser_MarshalUnmarshal(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	user := User{
		ID:        "u-1",
		Username:  "charlie",
		Email:     "charlie@example.com",
		Role:      "admin",
		CreatedAt: now,
	}

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got User
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.ID != user.ID {
		t.Errorf("ID = %q, want %q", got.ID, user.ID)
	}
	if got.Username != user.Username {
		t.Errorf("Username = %q, want %q", got.Username, user.Username)
	}
	if got.Email != user.Email {
		t.Errorf("Email = %q, want %q", got.Email, user.Email)
	}
	if got.Role != user.Role {
		t.Errorf("Role = %q, want %q", got.Role, user.Role)
	}
	if !got.CreatedAt.Equal(user.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, user.CreatedAt)
	}
}

// ---------------------------------------------------------------------------
// Problem marshaling
// ---------------------------------------------------------------------------

func TestProblem_MarshalUnmarshal(t *testing.T) {
	p := Problem{
		Type:      "https://encodeswarmr.dev/errors/not-found",
		Title:     "Not Found",
		Status:    404,
		Detail:    "job not found",
		Instance:  "/api/v1/jobs/j-missing",
		RequestID: "req-xyz",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Problem
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Type != p.Type {
		t.Errorf("Type = %q, want %q", got.Type, p.Type)
	}
	if got.Title != p.Title {
		t.Errorf("Title = %q, want %q", got.Title, p.Title)
	}
	if got.Status != p.Status {
		t.Errorf("Status = %d, want %d", got.Status, p.Status)
	}
	if got.Detail != p.Detail {
		t.Errorf("Detail = %q, want %q", got.Detail, p.Detail)
	}
	if got.Instance != p.Instance {
		t.Errorf("Instance = %q, want %q", got.Instance, p.Instance)
	}
	if got.RequestID != p.RequestID {
		t.Errorf("RequestID = %q, want %q", got.RequestID, p.RequestID)
	}
}

// ---------------------------------------------------------------------------
// Collection type
// ---------------------------------------------------------------------------

func TestCollection_Fields(t *testing.T) {
	items := []Agent{
		{ID: "ag-1", Name: "enc-01"},
		{ID: "ag-2", Name: "enc-02"},
	}
	col := Collection[Agent]{
		Items:      items,
		TotalCount: 100,
		NextCursor: "next-page-token",
	}
	if len(col.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(col.Items))
	}
	if col.TotalCount != 100 {
		t.Errorf("TotalCount = %d, want 100", col.TotalCount)
	}
	if col.NextCursor != "next-page-token" {
		t.Errorf("NextCursor = %q, want next-page-token", col.NextCursor)
	}
}

// ---------------------------------------------------------------------------
// Nullable *float64 precision
// ---------------------------------------------------------------------------

func TestNullableFloat64_Precision(t *testing.T) {
	// Ensure *float64 fields survive a JSON round-trip without precision loss.
	score := 98.765432
	src := Source{
		ID:        "src-fp",
		CreatedAt: time.Now(),
		VMafScore: &score,
	}
	data, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Source
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.VMafScore == nil {
		t.Fatal("VMafScore is nil after round-trip")
	}
	if *got.VMafScore != score {
		t.Errorf("VMafScore = %v, want %v", *got.VMafScore, score)
	}
}

// ---------------------------------------------------------------------------
// Nullable *int boundary values
// ---------------------------------------------------------------------------

func TestNullableInt_ZeroValue(t *testing.T) {
	// Zero is a valid exit code and must not be treated as absent.
	exit := 0
	task := Task{
		ID:        "t-exit0",
		JobID:     "j-1",
		ExitCode:  &exit,
		CreatedAt: time.Now(),
	}
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Task
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.ExitCode == nil {
		t.Fatal("ExitCode is nil; zero must be preserved")
	}
	if *got.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", *got.ExitCode)
	}
}

// ---------------------------------------------------------------------------
// JobDetail
// ---------------------------------------------------------------------------

func TestJobDetail_MarshalUnmarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	detail := JobDetail{
		Job: Job{
			ID:        "j-detail",
			Status:    JobCompleted,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Tasks: []Task{
			{ID: "t-1", JobID: "j-detail", Status: TaskCompleted, CreatedAt: now},
			{ID: "t-2", JobID: "j-detail", Status: TaskFailed, CreatedAt: now},
		},
	}

	data, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got JobDetail
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Job.ID != "j-detail" {
		t.Errorf("Job.ID = %q, want j-detail", got.Job.ID)
	}
	if len(got.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, want 2", len(got.Tasks))
	}
	if got.Tasks[0].ID != "t-1" {
		t.Errorf("Tasks[0].ID = %q, want t-1", got.Tasks[0].ID)
	}
	if got.Tasks[1].Status != TaskFailed {
		t.Errorf("Tasks[1].Status = %q, want %q", got.Tasks[1].Status, TaskFailed)
	}
}
