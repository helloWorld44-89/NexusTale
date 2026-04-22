import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
  label?: string
}

interface State {
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(`[ErrorBoundary:${this.props.label ?? 'unknown'}]`, error, info.componentStack)
  }

  handleReset = () => this.setState({ error: null })

  render() {
    if (this.state.error) {
      if (this.props.fallback) return this.props.fallback
      return (
        <div className="flex flex-col items-center justify-center h-full gap-4 p-8 text-center">
          <p className="text-brand-text-muted text-sm">
            Something went wrong in {this.props.label ?? 'this panel'}.
          </p>
          <p className="font-mono text-xs text-red-400 max-w-md break-words">
            {this.state.error.message}
          </p>
          <button
            onClick={this.handleReset}
            className="px-3 py-1 text-xs rounded bg-brand-surface border border-brand-border text-brand-text hover:bg-brand-surface/80"
          >
            Try again
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
