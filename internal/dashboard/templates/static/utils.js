// Global toast notification system
window.showToast = function(message, type = 'info', duration = 5000) {
    const container = document.getElementById('toast-container');
    if (!container) return;
    
    const toast = document.createElement('div');
    toast.className = 'toast-notification';
    
    const iconMap = {
        success: '<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path></svg>',
        error: '<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path></svg>',
        warning: '<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path></svg>',
        info: '<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"></path></svg>'
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
            <div class="flex items-start space-x-3 p-4">
                <div class="flex-shrink-0 ${colors.icon}">
                    ${iconMap[type] || iconMap.info}
                </div>
                <div class="flex-1 min-w-0">
                    <p class="text-sm font-medium">${message}</p>
                </div>
                <div class="flex-shrink-0">
                    <button class="toast-close-btn ${colors.text} hover:text-white transition-colors" aria-label="Close notification">
                        <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                        </svg>
                    </button>
                </div>
            </div>
            <div class="toast-progress ${colors.border}"></div>
        </div>
    `;
    
    // Add close functionality
    toast.querySelector('.toast-close-btn').addEventListener('click', () => {
        toast.classList.add('toast-exit');
        setTimeout(() => toast.remove(), 300);
    });
    
    container.appendChild(toast);
    
    // Trigger entrance animation
    requestAnimationFrame(() => {
        toast.classList.add('toast-enter');
    });
    
    // Auto-remove with progress bar
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
};

// Responsive utilities
window.isMobile = () => window.innerWidth < 640;
window.isTablet = () => window.innerWidth >= 640 && window.innerWidth < 1024;
window.isDesktop = () => window.innerWidth >= 1024;

// Format utilities
window.formatBytes = function(bytes, decimals = 2) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
};

window.formatDuration = function(seconds) {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = Math.floor(seconds % 60);
    
    const parts = [];
    if (days > 0) parts.push(`${days}d`);
    if (hours > 0) parts.push(`${hours}h`);
    if (minutes > 0) parts.push(`${minutes}m`);
    if (secs > 0) parts.push(`${secs}s`);
    
    return parts.join(' ') || '0s';
};

// Debounce utility
window.debounce = function(func, wait, immediate) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            timeout = null;
            if (!immediate) func.apply(this, args);
        };
        const callNow = immediate && !timeout;
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
        if (callNow) func.apply(this, args);
    };
};

// Theme utilities
window.setTheme = function(theme) {
    document.documentElement.className = theme;
    localStorage.setItem('theme', theme);
};

window.getTheme = function() {
    return localStorage.getItem('theme') || 'light';
};

// Copy to clipboard utility
window.copyToClipboard = function(text) {
    navigator.clipboard.writeText(text).then(() => {
        window.showToast('Copied to clipboard', 'success');
    }).catch(err => {
        window.showToast('Failed to copy to clipboard', 'error');
        console.error('Copy failed:', err);
    });
};