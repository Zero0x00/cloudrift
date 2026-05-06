import { Component, type ErrorInfo, type ReactNode } from "react";

type Props = {
  children: ReactNode;
  fallback?: (error: Error, reset: () => void) => ReactNode;
};

type State = { error: Error | null };

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[ErrorBoundary]", error, info.componentStack);
  }

  reset = () => this.setState({ error: null });

  render() {
    if (this.state.error) {
      if (this.props.fallback) {
        return this.props.fallback(this.state.error, this.reset);
      }
      return (
        <div className="m-4 rounded-lg border border-rose-300 bg-rose-50 p-4 dark:border-rose-900/70 dark:bg-rose-950/30">
          <p className="text-sm font-semibold text-rose-900 dark:text-rose-200">Render error</p>
          <pre className="mt-2 overflow-auto whitespace-pre-wrap text-xs text-rose-800 dark:text-rose-300">
            {this.state.error.message}
            {"\n"}
            {this.state.error.stack}
          </pre>
          <button
            type="button"
            className="mt-3 rounded border border-rose-300 px-2 py-1 text-xs text-rose-800 hover:bg-rose-100 dark:border-rose-800 dark:text-rose-300 dark:hover:bg-rose-900/40"
            onClick={this.reset}
          >
            Retry
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
