import React from 'react';
import { Link, useParams } from 'react-router-dom';
import { Home } from 'lucide-react';
import { useAuth } from '../../contexts/AuthContext';
import PictureCue from '../k2/PictureCue';
import ReadAloudButton from '../k2/ReadAloudButton';

const K2Layout = ({ children }) => {
  const { courseId } = useParams();
  const { user, logout } = useAuth();

  const firstName =
    user?.first_name || user?.short_name?.split(' ')[0] || user?.name?.split(' ')[0] || 'friend';
  const greeting = `Welcome back, ${firstName}! Ready to learn?`;

  const handleLogout = async () => {
    await logout();
    window.location.href = '/login';
  };

  return (
    <div className="min-h-screen bg-sky-50 flex flex-col items-center pb-28">
      <header className="w-full flex items-center justify-center py-4">
        <Link
          to={`/courses/${courseId}`}
          className="w-16 h-16 rounded-full bg-brand-500 flex items-center justify-center shadow-lg hover:bg-brand-600 transition-colors"
          aria-label="Home"
        >
          <Home className="w-8 h-8 text-white" />
        </Link>
      </header>

      {/* Friendly greeting with read-aloud */}
      <section className="w-full max-w-4xl px-4 flex items-center justify-center gap-4">
        <h1 className="font-display text-2xl sm:text-3xl text-slate-800 text-center">
          {greeting}
        </h1>
        <ReadAloudButton text={greeting} />
      </section>

      <main className="w-full max-w-4xl px-4 py-4 flex-1">{children}</main>

      {/* Kid-friendly bottom nav with picture cues */}
      <nav
        className="fixed bottom-0 inset-x-0 z-30 bg-slate-800 flex items-center justify-around px-4 py-3 shadow-[0_-4px_12px_rgba(0,0,0,0.15)]"
        role="navigation"
        aria-label="Global navigation"
      >
        <Link
          to="/"
          aria-label="My Classes"
          className="min-h-[44px] min-w-[44px] flex items-center justify-center rounded-xl px-3 py-1 text-white hover:bg-surface-0/10 transition-colors"
        >
          <PictureCue type="classes" label="My Classes" />
        </Link>
        <Link
          to={`/courses/${courseId}`}
          aria-label="Home"
          className="min-h-[44px] min-w-[44px] flex items-center justify-center rounded-xl px-3 py-1 bg-surface-0/15 text-white transition-colors"
        >
          <PictureCue type="home" label="Home" />
        </Link>
        <Link
          to="/inbox"
          aria-label="Messages"
          className="min-h-[44px] min-w-[44px] flex items-center justify-center rounded-xl px-3 py-1 text-white hover:bg-surface-0/10 transition-colors"
        >
          <PictureCue type="messages" label="Messages" />
        </Link>
        <button
          type="button"
          onClick={handleLogout}
          aria-label="Log Out"
          className="min-h-[44px] min-w-[44px] flex items-center justify-center rounded-xl px-3 py-1 text-white hover:bg-surface-0/10 transition-colors"
        >
          <PictureCue type="logout" label="Log Out" />
        </button>
      </nav>
    </div>
  );
};

export default K2Layout;
