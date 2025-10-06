import React, { Fragment } from 'react';
import { Listbox, Transition } from '@headlessui/react';
import { createPortal } from 'react-dom';
import clsx from 'clsx';

const Select = ({
  label,
  value,
  onChange,
  options = [],
  placeholder = 'Select an option',
  error,
  hint,
  disabled = false,
  className = '',
  containerClassName = '',
}) => {
  const selectedOption = options.find(opt => opt.value === value);
  const buttonRef = React.useRef(null);
  const [buttonRect, setButtonRect] = React.useState(null);

  React.useEffect(() => {
    if (buttonRef.current) {
      const rect = buttonRef.current.getBoundingClientRect();
      setButtonRect(rect);
    }
  }, []);

  const updatePosition = () => {
    if (buttonRef.current) {
      const rect = buttonRef.current.getBoundingClientRect();
      setButtonRect(rect);
    }
  };

  return (
    <div className={clsx('w-full max-w-full', containerClassName)}>
      {label && (
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          {label}
        </label>
      )}
      <Listbox value={value} onChange={onChange} disabled={disabled}>
        {({ open }) => {
          React.useEffect(() => {
            if (open) {
              updatePosition();
            }
          }, [open]);

          return (
            <div className="relative">
              <Listbox.Button
                ref={buttonRef}
                className={clsx(
                  'relative w-full min-h-[48px] h-auto px-3 pr-10 py-2 text-left text-sm sm:text-base rounded-lg border transition-colors duration-200',
                  'focus:outline-none focus:ring-2 focus:ring-offset-0',
                  'disabled:bg-gray-100 disabled:cursor-not-allowed dark:disabled:bg-gray-800',
                  'truncate overflow-hidden',
                  {
                    'border-red-500 focus:border-red-500 focus:ring-red-500': error,
                    'border-gray-300 focus:border-blue-500 focus:ring-blue-500 dark:border-gray-600 dark:focus:border-blue-400': !error,
                    'bg-white dark:bg-gray-900 text-gray-900 dark:text-white': !disabled,
                  },
                  className
                )}
                aria-invalid={error ? 'true' : 'false'}
              >
                <span className={clsx('block truncate pr-2', { 'text-gray-400 dark:text-gray-500': !selectedOption })}>
                  {selectedOption ? selectedOption.label : placeholder}
                </span>
                <span className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
                  <svg
                    className={clsx('h-4 w-4 sm:h-5 sm:w-5 text-gray-400 transition-transform flex-shrink-0', { 'rotate-180': open })}
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                    aria-hidden="true"
                  >
                    <path
                      fillRule="evenodd"
                      d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
                      clipRule="evenodd"
                    />
                  </svg>
                </span>
              </Listbox.Button>
              {open && buttonRect && createPortal(
                <Transition
                  show={open}
                  as={Fragment}
                  leave="transition ease-in duration-100"
                  leaveFrom="opacity-100"
                  leaveTo="opacity-0"
                >
                  <Listbox.Options
                    static
                    className="fixed z-[9999] max-h-60 overflow-auto rounded-lg bg-white dark:bg-gray-800 py-1 text-sm sm:text-base shadow-xl ring-1 ring-black ring-opacity-5 focus:outline-none"
                    style={{
                      top: `${buttonRect.bottom + window.scrollY + 4}px`,
                      left: `${buttonRect.left + window.scrollX}px`,
                      width: `${buttonRect.width}px`,
                      minWidth: '200px',
                      maxWidth: 'calc(100vw - 2rem)',
                    }}
                  >
                    {options.map((option) => (
                      <Listbox.Option
                        key={option.value}
                        value={option.value}
                        disabled={option.disabled}
                        className={({ active, selected }) =>
                          clsx(
                            'relative cursor-pointer select-none py-2 sm:py-3 pl-8 sm:pl-10 pr-3 sm:pr-4 min-h-[44px] flex items-center',
                            {
                              'bg-blue-100 dark:bg-blue-900': active,
                              'text-gray-900 dark:text-white': !active,
                              'opacity-50 cursor-not-allowed': option.disabled,
                            }
                          )
                        }
                      >
                        {({ selected }) => (
                          <>
                            <span className={clsx('block truncate break-words', { 'font-medium': selected })}>
                              {option.label}
                            </span>
                            {selected && (
                              <span className="absolute inset-y-0 left-0 flex items-center pl-2 sm:pl-3 text-blue-600 dark:text-blue-400">
                                <svg className="h-4 w-4 sm:h-5 sm:w-5" viewBox="0 0 20 20" fill="currentColor">
                                  <path
                                    fillRule="evenodd"
                                    d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                                    clipRule="evenodd"
                                  />
                                </svg>
                              </span>
                            )}
                          </>
                        )}
                      </Listbox.Option>
                    ))}
                  </Listbox.Options>
                </Transition>,
                document.body
              )}
            </div>
          );
        }}
      </Listbox>
      {error && (
        <p className="mt-2 text-sm text-red-600 dark:text-red-400">
          {error}
        </p>
      )}
      {hint && !error && (
        <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
          {hint}
        </p>
      )}
    </div>
  );
};

export default Select;
