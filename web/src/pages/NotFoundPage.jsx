import React from 'react';
import { Link } from 'react-router-dom';

const NotFoundPage = () => (
  <div className="min-h-screen bg-surface-1 flex items-center justify-center p-4">
    <div className="text-center max-w-md">
      <div className="text-6xl font-bold text-text-disabled mb-4">404</div>
      <h1 className="text-xl font-semibold text-text-primary mb-2">Page not found</h1>
      <p className="text-text-tertiary mb-6">
        The page you're looking for doesn't exist or has been moved.
      </p>
      <Link
        to="/"
        className="inline-block bg-brand-600 text-white px-6 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
      >
        Go to Dashboard
      </Link>
    </div>
  </div>
);

export default NotFoundPage;
