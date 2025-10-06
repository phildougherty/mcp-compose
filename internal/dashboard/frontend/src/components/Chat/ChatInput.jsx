import React, { useState, useRef, useEffect } from 'react';
import { Button } from '../shared';
import clsx from 'clsx';

export default function ChatInput({ onSend, disabled }) {
  const [inputMessage, setInputMessage] = useState('');
  const textareaRef = useRef(null);

  const autoResizeTextarea = () => {
    const textarea = textareaRef.current;
    if (!textarea) return;

    textarea.style.height = 'auto';
    const maxHeight = window.innerWidth < 768 ? 140 : 200;
    const newHeight = Math.min(textarea.scrollHeight, maxHeight);
    textarea.style.height = newHeight + 'px';
  };

  useEffect(() => {
    autoResizeTextarea();
  }, [inputMessage]);

  const handleSend = () => {
    if (!inputMessage.trim() || disabled) return;

    onSend(inputMessage);
    setInputMessage('');

    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
    }
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="input-wrapper flex gap-2 items-end">
      <textarea
        ref={textareaRef}
        value={inputMessage}
        onChange={(e) => setInputMessage(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="Message..."
        className={clsx(
          'message-input flex-1 resize-none rounded-lg border border-gray-300 dark:border-gray-600',
          'bg-white dark:bg-gray-800 text-gray-900 dark:text-white',
          'px-4 py-3 text-base focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-600',
          'placeholder:text-gray-500 dark:placeholder:text-gray-400 placeholder:text-sm',
          'min-h-[48px]',
          disabled && 'opacity-50 cursor-not-allowed'
        )}
        disabled={disabled}
        rows={1}
      />

      <Button
        onClick={handleSend}
        disabled={!inputMessage.trim() || disabled}
        variant="primary"
        className="send-btn min-h-[48px] min-w-[48px] px-3 flex items-center justify-center"
        title={disabled ? "Sending..." : "Send message"}
      >
        {disabled ? (
          <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-white" />
        ) : (
          <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"
            />
          </svg>
        )}
      </Button>
    </div>
  );
}
