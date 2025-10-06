/**
 * Copies text to the clipboard
 * @param {string} text - Text to copy to clipboard
 * @returns {Promise<boolean>} Promise that resolves to true if successful
 */
export async function copyToClipboard(text) {
  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text);

      return true;
    }

    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.left = '-999999px';
    textArea.style.top = '-999999px';
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();

    let success = false;
    try {
      success = document.execCommand('copy');
    } catch (err) {
      console.error('Fallback copy failed:', err);
    }

    document.body.removeChild(textArea);

    return success;
  } catch (err) {
    console.error('Copy to clipboard failed:', err);

    return false;
  }
}

/**
 * Reads text from the clipboard
 * @returns {Promise<string|null>} Promise that resolves to clipboard text or null
 */
export async function readFromClipboard() {
  try {
    if (navigator.clipboard && navigator.clipboard.readText) {
      const text = await navigator.clipboard.readText();

      return text;
    }

    return null;
  } catch (err) {
    console.error('Read from clipboard failed:', err);

    return null;
  }
}

/**
 * Checks if clipboard API is available
 * @returns {boolean} True if clipboard API is supported
 */
export function isClipboardSupported() {
  return !!(navigator.clipboard && navigator.clipboard.writeText);
}

/**
 * Copies text to clipboard with fallback and optional callback
 * @param {string} text - Text to copy
 * @param {Function} onSuccess - Success callback
 * @param {Function} onError - Error callback
 * @returns {Promise<void>}
 */
export async function copyWithCallback(text, onSuccess, onError) {
  try {
    const success = await copyToClipboard(text);

    if (success && onSuccess) {
      onSuccess();
    } else if (!success && onError) {
      onError(new Error('Copy failed'));
    }
  } catch (err) {
    if (onError) {
      onError(err);
    }
  }
}
