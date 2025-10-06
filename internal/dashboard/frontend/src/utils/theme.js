/**
 * Sets the theme for the application
 * @param {string} theme - Theme name ('light', 'dark', or custom theme)
 */
export function setTheme(theme) {
  document.documentElement.className = theme;
  localStorage.setItem('theme', theme);
}

/**
 * Gets the current theme from localStorage
 * @returns {string} The current theme name (default: 'light')
 */
export function getTheme() {
  return localStorage.getItem('theme') || 'light';
}

/**
 * Toggles between light and dark themes
 * @returns {string} The new theme name
 */
export function toggleTheme() {
  const currentTheme = getTheme();
  const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
  setTheme(newTheme);

  return newTheme;
}

/**
 * Initializes theme based on user preference or system preference
 * @param {string} defaultTheme - Default theme if no preference is found
 * @returns {string} The initialized theme name
 */
export function initializeTheme(defaultTheme = 'light') {
  const savedTheme = localStorage.getItem('theme');

  if (savedTheme) {
    setTheme(savedTheme);

    return savedTheme;
  }

  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  const theme = prefersDark ? 'dark' : defaultTheme;
  setTheme(theme);

  return theme;
}

/**
 * Listens for system theme changes and updates application theme
 * @param {Function} callback - Callback function called when system theme changes
 * @returns {Function} Cleanup function to remove the listener
 */
export function watchSystemTheme(callback) {
  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

  const handler = (e) => {
    const theme = e.matches ? 'dark' : 'light';
    setTheme(theme);
    if (callback) {
      callback(theme);
    }
  };

  if (mediaQuery.addEventListener) {
    mediaQuery.addEventListener('change', handler);

    return () => mediaQuery.removeEventListener('change', handler);
  }

  mediaQuery.addListener(handler);

  return () => mediaQuery.removeListener(handler);
}
