# Monaco Editor Integration

This document describes the Monaco Editor integration for the MCP Compose Workflow Builder.

## Overview

Monaco Editor (the editor that powers VS Code) has been integrated into the workflow builder to provide a full IDE experience for code editing in Code, Transform, and Decision nodes.

## Components Modified

### 1. MonacoCodeEditor Component
**Location:** `/internal/dashboard/frontend/src/components/common/MonacoCodeEditor.jsx`

A reusable wrapper component for Monaco Editor with the following features:

- Automatic theme detection (light/dark) matching dashboard theme
- Multi-language support (JavaScript, Python, Bash, Go, Ruby, PHP, JSON, Shell)
- Configurable height and options
- Syntax validation and auto-completion
- TypeScript definitions for `input` and `context` variables
- Minimap toggle
- Line numbers
- Custom loading state
- Responsive to theme changes via MutationObserver

#### Props:
- `value` (string): Current editor value
- `onChange` (function): Callback when value changes
- `language` (string): Programming language mode (default: 'javascript')
- `height` (string): Editor height (default: '400px')
- `readOnly` (boolean): Read-only mode (default: false)
- `showMinimap` (boolean): Show/hide minimap (default: true)
- `showLineNumbers` (boolean): Show/hide line numbers (default: true)
- `placeholder` (string): Placeholder text when empty
- `options` (object): Additional Monaco editor options

### 2. CodeNodeConfig Component
**Location:** `/internal/dashboard/frontend/src/components/WorkflowBuilder/config/CodeNodeConfig.jsx`

**Changes:**
- Replaced main code textarea with MonacoCodeEditor
- Language-specific syntax highlighting based on selected language
- 450px height with minimap enabled
- Environment variables editor with shell syntax (120px height, no minimap)

### 3. TransformNodeConfig Component
**Location:** `/internal/dashboard/frontend/src/components/WorkflowBuilder/config/TransformNodeConfig.jsx`

**Changes:**
- Replaced transform code textarea with MonacoCodeEditor
- JavaScript syntax highlighting and validation
- 350px height with minimap enabled
- Default value editor with JSON syntax (120px height, no minimap)

### 4. DecisionNodeConfig Component
**Location:** `/internal/dashboard/frontend/src/components/WorkflowBuilder/config/DecisionNodeConfig.jsx`

**Changes:**
- Replaced condition textarea with MonacoCodeEditor
- JavaScript syntax highlighting and validation
- 250px height with minimap disabled (simpler conditions)

## Features

### Auto-completion
The editor provides IntelliSense auto-completion for:
- JavaScript/TypeScript syntax
- Workflow-specific variables (`input`, `context`)
- Standard language features

### Syntax Validation
Real-time syntax validation for:
- JavaScript/TypeScript code
- Python code
- Bash scripts
- JSON data
- Other supported languages

### Theme Synchronization
The editor automatically detects and responds to dashboard theme changes:
- Light mode: Uses 'vs-light' theme
- Dark mode: Uses 'vs-dark' theme
- Listens to localStorage changes
- Observes DOM class changes for instant updates

### Type Definitions
For JavaScript/TypeScript, the editor includes type definitions:
```typescript
declare const input: any;
declare const context: {
  workflowId?: string;
  executionId?: string;
  [key: string]: any;
};
```

## Installation

Monaco Editor is installed via npm:
```bash
npm install @monaco-editor/react@4.7.0
```

## Usage Example

```jsx
import MonacoCodeEditor from '../../common/MonacoCodeEditor';

<MonacoCodeEditor
  value={code}
  onChange={(newValue) => setCode(newValue)}
  language="javascript"
  height="400px"
  showMinimap={true}
  showLineNumbers={true}
/>
```

## Browser Compatibility

Monaco Editor supports modern browsers:
- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest)

## Performance Notes

- Monaco Editor is loaded lazily on component mount
- First load may take 1-2 seconds to download editor assets
- Subsequent loads are cached by the browser
- The editor adds approximately 1.5MB to the bundle size (gzipped)

## Future Enhancements

Potential future improvements:
1. Add run/test functionality for code nodes
2. Implement real-time error highlighting
3. Add code snippets library
4. Support custom themes
5. Add collaborative editing features
6. Implement code formatting shortcuts
7. Add diff viewer for comparing code versions
