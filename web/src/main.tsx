import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { ThemeProvider } from './theme'
import { BrandingProvider } from './contexts/BrandingContext'
import ErrorBoundary from './components/ErrorBoundary'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <ThemeProvider>
        <BrandingProvider>
          <BrowserRouter>
            <App />
          </BrowserRouter>
        </BrandingProvider>
      </ThemeProvider>
    </ErrorBoundary>
  </React.StrictMode>,
)
