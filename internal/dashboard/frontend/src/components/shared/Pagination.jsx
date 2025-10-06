import React from 'react';
import clsx from 'clsx';

const Pagination = ({
  currentPage,
  totalPages,
  onPageChange,
  showFirstLast = true,
  maxVisiblePages = 5,
  className = '',
}) => {
  const getVisiblePages = () => {
    if (totalPages <= maxVisiblePages) {
      return Array.from({ length: totalPages }, (_, i) => i + 1);
    }

    const halfVisible = Math.floor(maxVisiblePages / 2);
    let start = Math.max(1, currentPage - halfVisible);
    let end = Math.min(totalPages, start + maxVisiblePages - 1);

    if (end - start + 1 < maxVisiblePages) {
      start = Math.max(1, end - maxVisiblePages + 1);
    }

    const pages = [];
    for (let i = start; i <= end; i++) {
      pages.push(i);
    }

    return pages;
  };

  const visiblePages = getVisiblePages();
  const showStartEllipsis = visiblePages[0] > 1;
  const showEndEllipsis = visiblePages[visiblePages.length - 1] < totalPages;

  const PageButton = ({ page, isActive = false, disabled = false, label, ariaLabel }) => (
    <button
      onClick={() => !disabled && onPageChange(page)}
      disabled={disabled}
      aria-label={ariaLabel || `Go to page ${page}`}
      aria-current={isActive ? 'page' : undefined}
      className={clsx(
        'min-h-[44px] min-w-[44px] px-4 py-2 text-sm font-medium rounded-lg',
        'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
        'transition-colors duration-200',
        {
          'bg-blue-600 text-white dark:bg-blue-500': isActive,
          'bg-white text-gray-700 hover:bg-gray-50 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700': !isActive && !disabled,
          'opacity-50 cursor-not-allowed': disabled,
          'border border-gray-300 dark:border-gray-600': !isActive,
        }
      )}
    >
      {label || page}
    </button>
  );

  return (
    <nav
      className={clsx('flex items-center justify-center gap-2', className)}
      aria-label="Pagination"
    >
      {showFirstLast && (
        <PageButton
          page={1}
          disabled={currentPage === 1}
          label={
            <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
              <path
                fillRule="evenodd"
                d="M15.707 15.707a1 1 0 01-1.414 0l-5-5a1 1 0 010-1.414l5-5a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 010 1.414zm-6 0a1 1 0 01-1.414 0l-5-5a1 1 0 010-1.414l5-5a1 1 0 011.414 1.414L5.414 10l4.293 4.293a1 1 0 010 1.414z"
                clipRule="evenodd"
              />
            </svg>
          }
          ariaLabel="Go to first page"
        />
      )}

      <PageButton
        page={Math.max(1, currentPage - 1)}
        disabled={currentPage === 1}
        label={
          <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
            <path
              fillRule="evenodd"
              d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z"
              clipRule="evenodd"
            />
          </svg>
        }
        ariaLabel="Go to previous page"
      />

      {showStartEllipsis && (
        <>
          <PageButton page={1} />
          <span className="px-2 text-gray-500 dark:text-gray-400">...</span>
        </>
      )}

      {visiblePages.map((page) => (
        <PageButton
          key={page}
          page={page}
          isActive={page === currentPage}
        />
      ))}

      {showEndEllipsis && (
        <>
          <span className="px-2 text-gray-500 dark:text-gray-400">...</span>
          <PageButton page={totalPages} />
        </>
      )}

      <PageButton
        page={Math.min(totalPages, currentPage + 1)}
        disabled={currentPage === totalPages}
        label={
          <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
            <path
              fillRule="evenodd"
              d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z"
              clipRule="evenodd"
            />
          </svg>
        }
        ariaLabel="Go to next page"
      />

      {showFirstLast && (
        <PageButton
          page={totalPages}
          disabled={currentPage === totalPages}
          label={
            <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
              <path
                fillRule="evenodd"
                d="M10.293 15.707a1 1 0 010-1.414L14.586 10l-4.293-4.293a1 1 0 111.414-1.414l5 5a1 1 0 010 1.414l-5 5a1 1 0 01-1.414 0z"
                clipRule="evenodd"
              />
              <path
                fillRule="evenodd"
                d="M4.293 15.707a1 1 0 010-1.414L8.586 10 4.293 5.707a1 1 0 011.414-1.414l5 5a1 1 0 010 1.414l-5 5a1 1 0 01-1.414 0z"
                clipRule="evenodd"
              />
            </svg>
          }
          ariaLabel="Go to last page"
        />
      )}
    </nav>
  );
};

export default Pagination;
