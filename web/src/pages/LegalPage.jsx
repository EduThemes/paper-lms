import React from 'react';
import { Link } from 'react-router-dom';
import useDocumentTitle from '../hooks/useDocumentTitle';
import BrandLogo from '../components/brand/BrandLogo';

// Static legal / policy pages (G163). The login page and the COPPA consent
// page link here; they are public so a parent or a signed-out visitor can
// read them. The text describes how this Paper LMS deployment handles data
// in general terms; the operating institution's own policies govern.

const PAGES = {
  privacy: {
    title: 'Privacy policy',
    sections: [
      ['Who runs this site', 'This learning platform is operated by your school, district or institution (the "operator") using Paper LMS software. The operator decides what data is collected and how long it is kept, and is the party to contact about your records.'],
      ['What we collect', 'Account details (name, email, login), course enrollments, the work you submit, grades and feedback, discussion and inbox messages, and basic usage logs (pages visited, sign-in times) needed to run and secure the service.'],
      ['How it is used', 'To deliver courses, record and report grades, send the notifications you choose, keep the service secure and meet legal obligations. Education records are not sold and are not used for advertising.'],
      ['Student records (FERPA)', 'Education records are shared only with school officials who have a legitimate educational interest, with parents or eligible students, or as the law allows. Students and parents can ask the operator to inspect or correct records, or to export their data from Account settings.'],
      ['Children under 13 (COPPA)', 'Where the operator enables child-safe mode, accounts for children under 13 require verifiable parental consent before the child can use the service. Parents may review, delete or withdraw consent for their child\'s information by contacting the operator.'],
      ['Retention and deletion', 'Data is kept for as long as the operator\'s retention policy requires. Deletion requests are handled by the operator\'s administrators.'],
      ['Contact', 'Questions about privacy go to your school or institution\'s administrator.'],
    ],
  },
  terms: {
    title: 'Terms of use',
    sections: [
      ['Acceptable use', 'Use the platform for learning and teaching. Do not share your password, access another person\'s account, upload malware, or post content that is unlawful, harassing or infringes someone else\'s rights.'],
      ['Your content', 'You keep the rights to the work you submit. You allow the operator to store and display it to the people your course setup permits (instructors, classmates where the activity calls for it).'],
      ['Academic integrity', 'Your institution\'s academic integrity and conduct policies apply to everything you do here.'],
      ['Availability', 'The operator aims to keep the service available but it may be interrupted for maintenance or problems outside its control.'],
      ['Changes', 'The operator may update these terms; continued use after an update means you accept it.'],
    ],
  },
  accessibility: {
    title: 'Accessibility',
    sections: [
      ['Our goal', 'Paper LMS aims to meet WCAG 2.1 level AA. Pages support keyboard navigation, screen readers, browser zoom and reduced-motion settings, and each page announces its title when you navigate.'],
      ['Reading preferences', 'You can change text size, spacing and dyslexia-friendly fonts in your reading preferences.'],
      ['Report a barrier', 'If something is hard to use with your assistive technology, tell your instructor or your institution\'s administrator so it can be fixed or an alternative provided.'],
    ],
  },
};

export default function LegalPage({ kind }) {
  const page = PAGES[kind] || PAGES.privacy;
  useDocumentTitle(page.title);
  return (
    <div className="min-h-screen bg-surface-1">
      <header className="border-b border-border-default bg-surface-0">
        <div className="mx-auto flex max-w-3xl items-center justify-between px-6 py-4">
          <Link to="/" className="flex items-center gap-2 text-text-primary" aria-label="Paper LMS home">
            <BrandLogo size={28} />
          </Link>
          <nav aria-label="Policies" className="flex gap-4 text-sm">
            <Link to="/privacy" className="text-brand-600 hover:underline">Privacy</Link>
            <Link to="/terms" className="text-brand-600 hover:underline">Terms</Link>
            <Link to="/accessibility" className="text-brand-600 hover:underline">Accessibility</Link>
          </nav>
        </div>
      </header>
      <main id="main-content" className="mx-auto max-w-3xl px-6 py-10">
        <h1 className="mb-6 text-3xl font-bold text-text-primary">{page.title}</h1>
        <div className="space-y-6">
          {page.sections.map(([heading, body]) => (
            <section key={heading}>
              <h2 className="mb-1 text-lg font-semibold text-text-primary">{heading}</h2>
              <p className="text-sm leading-relaxed text-text-secondary">{body}</p>
            </section>
          ))}
        </div>
      </main>
    </div>
  );
}
