/**
 * Validates an email address
 * @param {string} email - Email address to validate
 * @returns {boolean} True if email is valid
 */
export function isValidEmail(email) {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

  return emailRegex.test(email);
}

/**
 * Validates a URL
 * @param {string} url - URL to validate
 * @returns {boolean} True if URL is valid
 */
export function isValidUrl(url) {
  try {
    new URL(url);

    return true;
  } catch {
    return false;
  }
}

/**
 * Validates a cron expression
 * @param {string} cron - Cron expression to validate
 * @returns {boolean} True if cron expression is valid
 */
export function isValidCron(cron) {
  const cronRegex = /^(\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([0-9]|1[0-9]|2[0-3])|\*\/([0-9]|1[0-9]|2[0-3])) (\*|([1-9]|1[0-9]|2[0-9]|3[0-1])|\*\/([1-9]|1[0-9]|2[0-9]|3[0-1])) (\*|([1-9]|1[0-2])|\*\/([1-9]|1[0-2])) (\*|([0-6])|\*\/([0-6]))$/;

  return cronRegex.test(cron.trim());
}

/**
 * Validates a port number
 * @param {number|string} port - Port number to validate
 * @returns {boolean} True if port is valid
 */
export function isValidPort(port) {
  const portNum = parseInt(port, 10);

  return !isNaN(portNum) && portNum > 0 && portNum <= 65535;
}

/**
 * Validates an IP address (IPv4)
 * @param {string} ip - IP address to validate
 * @returns {boolean} True if IP is valid
 */
export function isValidIPv4(ip) {
  const ipRegex = /^(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;

  return ipRegex.test(ip);
}

/**
 * Validates a required field (non-empty string)
 * @param {string} value - Value to validate
 * @returns {boolean} True if value is not empty
 */
export function isRequired(value) {
  return typeof value === 'string' && value.trim().length > 0;
}

/**
 * Validates string length
 * @param {string} value - String to validate
 * @param {number} min - Minimum length
 * @param {number} max - Maximum length
 * @returns {boolean} True if length is within range
 */
export function isValidLength(value, min, max) {
  const length = value ? value.length : 0;

  return length >= min && length <= max;
}

/**
 * Validates a JSON string
 * @param {string} jsonString - JSON string to validate
 * @returns {boolean} True if JSON is valid
 */
export function isValidJSON(jsonString) {
  try {
    JSON.parse(jsonString);

    return true;
  } catch {
    return false;
  }
}

/**
 * Validates a domain name
 * @param {string} domain - Domain name to validate
 * @returns {boolean} True if domain is valid
 */
export function isValidDomain(domain) {
  const domainRegex = /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$/i;

  return domainRegex.test(domain);
}

/**
 * Validates a phone number (basic international format)
 * @param {string} phone - Phone number to validate
 * @returns {boolean} True if phone is valid
 */
export function isValidPhone(phone) {
  const phoneRegex = /^\+?[1-9]\d{1,14}$/;

  return phoneRegex.test(phone.replace(/[\s()-]/g, ''));
}

/**
 * Validates a password strength
 * @param {string} password - Password to validate
 * @param {Object} requirements - Requirements object
 * @returns {Object} Validation result with isValid and errors
 */
export function validatePassword(password, requirements = {}) {
  const {
    minLength = 8,
    requireUppercase = true,
    requireLowercase = true,
    requireNumbers = true,
    requireSpecialChars = true
  } = requirements;

  const errors = [];

  if (password.length < minLength) {
    errors.push(`Password must be at least ${minLength} characters`);
  }

  if (requireUppercase && !/[A-Z]/.test(password)) {
    errors.push('Password must contain at least one uppercase letter');
  }

  if (requireLowercase && !/[a-z]/.test(password)) {
    errors.push('Password must contain at least one lowercase letter');
  }

  if (requireNumbers && !/\d/.test(password)) {
    errors.push('Password must contain at least one number');
  }

  if (requireSpecialChars && !/[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]/.test(password)) {
    errors.push('Password must contain at least one special character');
  }

  return {
    isValid: errors.length === 0,
    errors
  };
}
