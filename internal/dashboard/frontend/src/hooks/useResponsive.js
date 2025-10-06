import { useState, useEffect, useCallback } from 'react';

const BREAKPOINTS = {
  xs: 320,
  sm: 640,
  md: 768,
  lg: 1024,
  xl: 1280,
  '2xl': 1536
};

/**
 * Custom hook for responsive breakpoint detection
 *
 * @param {Object} customBreakpoints - Custom breakpoint configuration (optional)
 * @returns {Object} Responsive state with device type flags
 */
export function useResponsive(customBreakpoints = {}) {
  const breakpoints = { ...BREAKPOINTS, ...customBreakpoints };

  const getDeviceType = useCallback((width) => {
    if (width < breakpoints.sm) {
      return 'mobile';
    } else if (width >= breakpoints.sm && width < breakpoints.lg) {
      return 'tablet';
    } else {
      return 'desktop';
    }
  }, [breakpoints]);

  const getResponsiveState = useCallback(() => {
    if (typeof window === 'undefined') {
      return {
        width: 0,
        height: 0,
        isMobile: false,
        isTablet: false,
        isDesktop: true,
        deviceType: 'desktop',
        isXs: false,
        isSm: false,
        isMd: false,
        isLg: false,
        isXl: false,
        is2Xl: false
      };
    }

    const width = window.innerWidth;
    const height = window.innerHeight;
    const deviceType = getDeviceType(width);

    return {
      width,
      height,
      isMobile: deviceType === 'mobile',
      isTablet: deviceType === 'tablet',
      isDesktop: deviceType === 'desktop',
      deviceType,
      isXs: width >= breakpoints.xs && width < breakpoints.sm,
      isSm: width >= breakpoints.sm && width < breakpoints.md,
      isMd: width >= breakpoints.md && width < breakpoints.lg,
      isLg: width >= breakpoints.lg && width < breakpoints.xl,
      isXl: width >= breakpoints.xl && width < breakpoints['2xl'],
      is2Xl: width >= breakpoints['2xl']
    };
  }, [breakpoints, getDeviceType]);

  const [responsiveState, setResponsiveState] = useState(getResponsiveState);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    const handleResize = () => {
      setResponsiveState(getResponsiveState());
    };

    handleResize();

    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
    };
  }, [getResponsiveState]);

  return responsiveState;
}

/**
 * Custom hook to check if screen width is at least a certain breakpoint
 *
 * @param {string} breakpoint - Breakpoint name ('xs', 'sm', 'md', 'lg', 'xl', '2xl')
 * @returns {boolean} True if screen width is at or above the breakpoint
 */
export function useMediaQuery(breakpoint) {
  const { width } = useResponsive();

  if (typeof window === 'undefined') {
    return false;
  }

  const breakpointValue = BREAKPOINTS[breakpoint];

  if (breakpointValue === undefined) {
    console.warn(`Invalid breakpoint: ${breakpoint}`);
    return false;
  }

  return width >= breakpointValue;
}

/**
 * Custom hook for orientation detection
 *
 * @returns {Object} Orientation state
 */
export function useOrientation() {
  const [orientation, setOrientation] = useState(() => {
    if (typeof window === 'undefined') {
      return 'landscape';
    }

    return window.innerHeight > window.innerWidth ? 'portrait' : 'landscape';
  });

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    const handleOrientationChange = () => {
      setOrientation(window.innerHeight > window.innerWidth ? 'portrait' : 'landscape');
    };

    handleOrientationChange();

    window.addEventListener('resize', handleOrientationChange);
    window.addEventListener('orientationchange', handleOrientationChange);

    return () => {
      window.removeEventListener('resize', handleOrientationChange);
      window.removeEventListener('orientationchange', handleOrientationChange);
    };
  }, []);

  return {
    orientation,
    isPortrait: orientation === 'portrait',
    isLandscape: orientation === 'landscape'
  };
}
