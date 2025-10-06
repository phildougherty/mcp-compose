import React from 'react';
import clsx from 'clsx';

const Card = ({
  children,
  variant = 'default',
  padding = 'md',
  hoverable = false,
  clickable = false,
  onClick,
  className = '',
  header,
  footer,
  ...props
}) => {
  const baseClasses = 'rounded-xl bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 transition-all duration-200';

  const variantClasses = {
    default: 'shadow-sm',
    outlined: 'border-2 shadow-none',
    elevated: 'shadow-md',
    flat: 'border-0 shadow-none',
  };

  const paddingClasses = {
    none: '',
    sm: 'p-3',
    md: 'p-4',
    lg: 'p-6',
    xl: 'p-8',
  };

  const interactiveClasses = clsx({
    'hover:shadow-lg hover:border-gray-300 dark:hover:border-gray-600 hover:-translate-y-0.5': hoverable,
    'cursor-pointer active:scale-[0.99] active:shadow-sm': clickable,
  });

  const cardClasses = clsx(
    baseClasses,
    variantClasses[variant],
    interactiveClasses,
    className
  );

  const contentClasses = clsx(paddingClasses[padding]);

  const CardWrapper = clickable ? 'button' : 'div';

  return (
    <CardWrapper
      className={cardClasses}
      onClick={clickable ? onClick : undefined}
      {...props}
    >
      {header && (
        <div className={clsx('border-b border-gray-200 dark:border-gray-700', paddingClasses[padding])}>
          {header}
        </div>
      )}
      <div className={contentClasses}>
        {children}
      </div>
      {footer && (
        <div className={clsx('border-t border-gray-200 dark:border-gray-700', paddingClasses[padding])}>
          {footer}
        </div>
      )}
    </CardWrapper>
  );
};

export default Card;
