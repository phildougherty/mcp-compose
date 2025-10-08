import React, { useEffect, useState } from 'react';
import Editor from '@monaco-editor/react';
import { getTheme } from '../../utils/theme';

const MonacoCodeEditor = ({
  value,
  onChange,
  language = 'javascript',
  height = '400px',
  readOnly = false,
  showMinimap = true,
  showLineNumbers = true,
  placeholder = '',
  options = {},
}) => {
  const [editorTheme, setEditorTheme] = useState('vs-light');

  useEffect(() => {
    const theme = getTheme();
    setEditorTheme(theme === 'dark' ? 'vs-dark' : 'vs-light');

    const handleStorageChange = (e) => {
      if (e.key === 'theme') {
        setEditorTheme(e.newValue === 'dark' ? 'vs-dark' : 'vs-light');
      }
    };

    window.addEventListener('storage', handleStorageChange);

    const observer = new MutationObserver((mutations) => {
      mutations.forEach((mutation) => {
        if (mutation.attributeName === 'class') {
          const currentTheme = document.documentElement.className;
          setEditorTheme(currentTheme === 'dark' ? 'vs-dark' : 'vs-light');
        }
      });
    });

    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    });

    return () => {
      window.removeEventListener('storage', handleStorageChange);
      observer.disconnect();
    };
  }, []);

  const defaultOptions = {
    minimap: { enabled: showMinimap },
    lineNumbers: showLineNumbers ? 'on' : 'off',
    readOnly,
    automaticLayout: true,
    scrollBeyondLastLine: false,
    fontSize: 13,
    tabSize: 2,
    wordWrap: 'on',
    formatOnPaste: true,
    formatOnType: true,
    autoClosingBrackets: 'always',
    autoClosingQuotes: 'always',
    suggestOnTriggerCharacters: true,
    quickSuggestions: true,
    folding: true,
    foldingStrategy: 'auto',
    showFoldingControls: 'always',
    matchBrackets: 'always',
    glyphMargin: true,
    contextmenu: true,
    mouseWheelZoom: true,
    smoothScrolling: true,
    cursorBlinking: 'smooth',
    renderWhitespace: 'selection',
    renderLineHighlight: 'all',
    ...options,
  };

  const handleEditorChange = (newValue) => {
    if (onChange && !readOnly) {
      onChange(newValue || '');
    }
  };

  const handleEditorDidMount = (editor, monaco) => {
    if (language === 'javascript' || language === 'typescript') {
      monaco.languages.typescript.javascriptDefaults.setDiagnosticsOptions({
        noSemanticValidation: false,
        noSyntaxValidation: false,
      });

      monaco.languages.typescript.javascriptDefaults.setCompilerOptions({
        target: monaco.languages.typescript.ScriptTarget.ES2020,
        allowNonTsExtensions: true,
        moduleResolution: monaco.languages.typescript.ModuleResolutionKind.NodeJs,
        module: monaco.languages.typescript.ModuleKind.CommonJS,
        noEmit: true,
        esModuleInterop: true,
        jsx: monaco.languages.typescript.JsxEmit.React,
        allowJs: true,
        typeRoots: ['node_modules/@types'],
      });

      const inputType = `
        declare const input: any;
        declare const context: {
          workflowId?: string;
          executionId?: string;
          [key: string]: any;
        };
      `;

      monaco.languages.typescript.javascriptDefaults.addExtraLib(
        inputType,
        'ts:workflow-types.d.ts'
      );
    }

    if (!value && placeholder) {
      const model = editor.getModel();
      if (model) {
        model.setValue(placeholder);
      }
    }
  };

  const beforeMount = (monaco) => {
    monaco.languages.registerCompletionItemProvider('javascript', {
      provideCompletionItems: (model, position) => {
        const word = model.getWordUntilPosition(position);
        const range = {
          startLineNumber: position.lineNumber,
          endLineNumber: position.lineNumber,
          startColumn: word.startColumn,
          endColumn: word.endColumn,
        };

        const suggestions = [
          {
            label: 'input',
            kind: monaco.languages.CompletionItemKind.Variable,
            insertText: 'input',
            detail: 'Input data from previous node',
            documentation: 'The data passed from the previous workflow node',
            range: range,
          },
          {
            label: 'context',
            kind: monaco.languages.CompletionItemKind.Variable,
            insertText: 'context',
            detail: 'Workflow execution context',
            documentation: 'Contains workflowId, executionId, and other workflow metadata',
            range: range,
          },
        ];

        return { suggestions: suggestions };
      },
    });
  };

  return (
    <div className="border border-gray-300 dark:border-gray-600 rounded-lg overflow-hidden">
      <Editor
        height={height}
        language={language}
        value={value}
        theme={editorTheme}
        onChange={handleEditorChange}
        onMount={handleEditorDidMount}
        beforeMount={beforeMount}
        options={defaultOptions}
        loading={
          <div className="flex items-center justify-center h-full bg-white dark:bg-gray-800">
            <div className="text-gray-500 dark:text-gray-400">Loading editor...</div>
          </div>
        }
      />
    </div>
  );
};

export default MonacoCodeEditor;
