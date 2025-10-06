/**
 * Shows a toast notification
 * @param {string} message - Message to display
 * @param {string} type - Toast type ('success', 'error', 'warning', 'info')
 * @param {number} duration - Duration in milliseconds (default: 3000)
 * @returns {HTMLElement|null} The toast element or null if container not found
 */
export function showToast(message, type = 'info', duration = 3000) {
  const container = document.getElementById('toast-container');
  if (!container) {
    console.warn('Toast container not found');

    return null;
  }

  const toast = document.createElement('div');
  toast.className = 'toast-notification';

  const iconMap = {
    success: '<svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path></svg>',
    error: '<svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path></svg>',
    warning: '<svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path></svg>',
    info: '<svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"></path></svg>'
  };

  const colorMap = {
    success: {
      bg: 'bg-green-800',
      border: 'border-green-600',
      icon: 'text-green-400',
      text: 'text-green-100'
    },
    error: {
      bg: 'bg-red-800',
      border: 'border-red-600',
      icon: 'text-red-400',
      text: 'text-red-100'
    },
    warning: {
      bg: 'bg-yellow-800',
      border: 'border-yellow-600',
      icon: 'text-yellow-400',
      text: 'text-yellow-100'
    },
    info: {
      bg: 'bg-blue-800',
      border: 'border-blue-600',
      icon: 'text-blue-400',
      text: 'text-blue-100'
    }
  };

  const colors = colorMap[type] || colorMap.info;

  toast.innerHTML = `
    <div class="toast-content ${colors.bg} ${colors.border} ${colors.text}">
      <div class="flex items-center space-x-2 px-3 py-2">
        <div class="flex-shrink-0 ${colors.icon}">
          ${iconMap[type] || iconMap.info}
        </div>
        <div class="flex-1 min-w-0">
          <p class="text-xs">${message}</p>
        </div>
        <div class="flex-shrink-0">
          <button class="toast-close-btn ${colors.text} hover:text-white transition-colors" aria-label="Close notification">
            <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
            </svg>
          </button>
        </div>
      </div>
      <div class="toast-progress ${colors.border}"></div>
    </div>
  `;

  toast.querySelector('.toast-close-btn').addEventListener('click', () => {
    toast.classList.add('toast-exit');
    setTimeout(() => toast.remove(), 300);
  });

  container.appendChild(toast);

  requestAnimationFrame(() => {
    toast.classList.add('toast-enter');
  });

  const progressBar = toast.querySelector('.toast-progress');
  if (progressBar) {
    progressBar.style.animation = `toast-progress ${duration}ms linear`;
  }

  setTimeout(() => {
    if (toast.parentNode) {
      toast.classList.add('toast-exit');
      setTimeout(() => toast.remove(), 300);
    }
  }, duration);

  return toast;
}

/**
 * Shows a success toast
 * @param {string} message - Message to display
 * @param {number} duration - Duration in milliseconds
 * @returns {HTMLElement|null} The toast element
 */
export function showSuccessToast(message, duration = 3000) {
  return showToast(message, 'success', duration);
}

/**
 * Shows an error toast
 * @param {string} message - Message to display
 * @param {number} duration - Duration in milliseconds
 * @returns {HTMLElement|null} The toast element
 */
export function showErrorToast(message, duration = 3000) {
  return showToast(message, 'error', duration);
}

/**
 * Shows a warning toast
 * @param {string} message - Message to display
 * @param {number} duration - Duration in milliseconds
 * @returns {HTMLElement|null} The toast element
 */
export function showWarningToast(message, duration = 3000) {
  return showToast(message, 'warning', duration);
}

/**
 * Shows an info toast
 * @param {string} message - Message to display
 * @param {number} duration - Duration in milliseconds
 * @returns {HTMLElement|null} The toast element
 */
export function showInfoToast(message, duration = 3000) {
  return showToast(message, 'info', duration);
}

/**
 * Clears all visible toasts
 */
export function clearAllToasts() {
  const container = document.getElementById('toast-container');
  if (container) {
    container.innerHTML = '';
  }
}
