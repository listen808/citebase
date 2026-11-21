export function getCommonEditorOptions() {
  return {
    fontSize: 14,
    fontFamily: "'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace",
    minimap: { enabled: false },
    wordWrap: 'on' as const,
    scrollBeyondLastLine: false,
    automaticLayout: true,
    lineNumbers: 'on' as const,
    renderLineHighlight: 'line' as const,
    scrollbar: { verticalScrollbarSize: 8, horizontalScrollbarSize: 8 },
    padding: { top: 12 },
    tabSize: 4,
  };
}
