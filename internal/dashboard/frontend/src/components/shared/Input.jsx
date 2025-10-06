import React from 'react';
import clsx from 'clsx';

const Input = React.forwardRef(({
  label,
  error,
  hint,
  type = 'text',
  disabled = false,
  className = '',
  containerClassName = '',
  leftIcon,
  rightIcon,
  ...props
}, ref) => {
  const inputClasses = clsx(
    'w-full h-12 px-4 text-base rounded-lg border transition-colors duration-200',
    'placeholder:text-gray-400 dark:placeholder:text-gray-500',
    'focus:outline-none focus:ring-2 focus:ring-offset-0',
    'disabled:bg-gray-100 disabled:cursor-not-allowed dark:disabled:bg-gray-800',
    {
      'border-red-500 focus:border-red-500 focus:ring-red-500': error,
      'border-gray-300 focus:border-blue-500 focus:ring-blue-500 dark:border-gray-600 dark:focus:border-blue-400': !error,
      'bg-white dark:bg-gray-900 text-gray-900 dark:text-white': !disabled,
      'pl-11': leftIcon,
      'pr-11': rightIcon,
    },
    className
  );

  return (
    <div className={clsx('w-full', containerClassName)}>
      {label && (
        <label
          className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
        >
          {label}
        </label>
      )}
      <div className="relative">
        {leftIcon && (
          <div className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500">
            {leftIcon}
          </div>
        )}
        <input
          ref={ref}
          type={type}
          disabled={disabled}
          className={inputClasses}
          aria-invalid={error ? 'true' : 'false'}
          aria-describedby={error ? `${props.id}-error` : hint ? `${props.id}-hint` : undefined}
          {...props}
        />
        {rightIcon && (
          <div className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500">
            {rightIcon}
          </div>
        )}
      </div>
      {error && (
        <p
          id={`${props.id}-error`}
          className="mt-2 text-sm text-red-600 dark:text-red-400"
        >
          {error}
        </p>
      )}
      {hint && !error && (
        <p
          id={`${props.id}-hint`}
          className="mt-2 text-sm text-gray-500 dark:text-gray-400"
        >
          {hint}
        </p>
      )}
    </div>
  );
});

Input.displayName = 'Input';

export default Input;
