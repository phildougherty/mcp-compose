/**
 * Checks if the current viewport is mobile size
 * @returns {boolean} True if viewport width is less than 640px
 */
export function isMobile() {
  return window.innerWidth < 640;
}

/**
 * Checks if the current viewport is tablet size
 * @returns {boolean} True if viewport width is between 640px and 1024px
 */
export function isTablet() {
  return window.innerWidth >= 640 && window.innerWidth < 1024;
}

/**
 * Checks if the current viewport is desktop size
 * @returns {boolean} True if viewport width is 1024px or greater
 */
export function isDesktop() {
  return window.innerWidth >= 1024;
}

/**
 * Gets the current breakpoint name based on viewport width
 * @returns {string} Breakpoint name ('xs', 'sm', 'md', 'lg', 'xl', '2xl')
 */
export function getCurrentBreakpoint() {
  const width = window.innerWidth;

  if (width < 320) return 'xs';
  if (width < 640) return 'sm';
  if (width < 768) return 'md';
  if (width < 1024) return 'lg';
  if (width < 1280) return 'xl';

  return '2xl';
}

/**
 * Checks if the device supports touch events
 * @returns {boolean} True if touch events are supported
 */
export function isTouchDevice() {
  return 'ontouchstart' in window || navigator.maxTouchPoints > 0;
}

/**
 * Checks if the user prefers reduced motion
 * @returns {boolean} True if user prefers reduced motion
 */
export function prefersReducedMotion() {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

/**
 * Checks if the user prefers dark mode
 * @returns {boolean} True if user prefers dark mode
 */
export function prefersDarkMode() {
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

/**
 * Gets the device pixel ratio
 * @returns {number} Device pixel ratio
 */
export function getDevicePixelRatio() {
  return window.devicePixelRatio || 1;
}

/**
 * Checks if the device is in landscape orientation
 * @returns {boolean} True if device is in landscape
 */
export function isLandscape() {
  return window.innerWidth > window.innerHeight;
}

/**
 * Checks if the device is in portrait orientation
 * @returns {boolean} True if device is in portrait
 */
export function isPortrait() {
  return window.innerHeight > window.innerWidth;
}

/**
 * Gets viewport dimensions
 * @returns {Object} Object with width and height properties
 */
export function getViewportDimensions() {
  return {
    width: window.innerWidth,
    height: window.innerHeight
  };
}
