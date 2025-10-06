import React from 'react';
import { Switch } from '@headlessui/react';
import clsx from 'clsx';

const Checkbox = ({
  label,
  description,
  checked,
  onChange,
  disabled = false,
  variant = 'checkbox',
  className = '',
  containerClassName = '',
}) => {
  if (variant === 'switch') {
    return (
      <Switch.Group as="div" className={clsx('flex items-center', containerClassName)}>
        <Switch
          checked={checked}
          onChange={onChange}
          disabled={disabled}
          className={clsx(
            'relative inline-flex h-8 w-14 items-center rounded-full transition-all duration-200',
            'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
            'active:scale-95 cursor-pointer',
            {
              'bg-blue-600 dark:bg-blue-500': checked,
              'bg-gray-300 dark:bg-gray-700': !checked,
              'opacity-50 cursor-not-allowed': disabled,
            }
          )}
        >
          <span
            className={clsx(
              'inline-block h-6 w-6 transform rounded-full bg-white transition-transform duration-200',
              'shadow-md',
              {
                'translate-x-7': checked,
                'translate-x-1': !checked,
              }
            )}
          />
        </Switch>
        {(label || description) && (
          <div className="ml-3 flex-1">
            {label && (
              <Switch.Label className="text-sm font-medium text-gray-900 dark:text-white cursor-pointer">
                {label}
              </Switch.Label>
            )}
            {description && (
              <Switch.Description className="text-sm text-gray-500 dark:text-gray-400">
                {description}
              </Switch.Description>
            )}
          </div>
        )}
      </Switch.Group>
    );
  }

  return (
    <div className={clsx('relative flex items-start min-h-[44px]', containerClassName)}>
      <div className="flex h-11 items-center">
        <input
          type="checkbox"
          checked={checked}
          onChange={(e) => onChange(e.target.checked)}
          disabled={disabled}
          className={clsx(
            'w-4 h-4 rounded border-2 text-blue-600 cursor-pointer',
            'border-gray-300 dark:border-gray-600',
            'bg-white dark:bg-gray-800',
            'focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
            'checked:bg-blue-600 checked:border-blue-600 dark:checked:bg-blue-500',
            'hover:border-blue-500 dark:hover:border-blue-400',
            'disabled:opacity-50 disabled:cursor-not-allowed',
            'transition-all duration-200 active:scale-95',
            className
          )}
        />
      </div>
      {(label || description) && (
        <div className="ml-3 flex-1">
          {label && (
            <label className="text-sm font-medium text-gray-900 dark:text-white cursor-pointer">
              {label}
            </label>
          )}
          {description && (
            <p className="text-sm text-gray-500 dark:text-gray-400">
              {description}
            </p>
          )}
        </div>
      )}
    </div>
  );
};

export default Checkbox;
