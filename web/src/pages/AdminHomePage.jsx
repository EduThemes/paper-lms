import React from 'react';
import { Link } from 'react-router-dom';
import {
  BookOpen, Users, Sliders, FileText, GraduationCap, ClipboardCheck, Upload,
  KeyRound, Shield, UserCog, RefreshCw, Bell, Code, Flag,
} from 'lucide-react';
import Layout from '../components/Layout';

const tiles = [
  { to: '/admin/courses',          icon: BookOpen,        label: 'Courses',         desc: 'Browse all courses, create new' },
  { to: '/admin/people',           icon: Users,           label: 'People',          desc: 'Users, roles, admins' },
  { to: '/admin/settings',         icon: Sliders,         label: 'Settings',        desc: 'Upload limits and account-wide knobs' },
  { to: '/admin/feature_flags',    icon: Flag,            label: 'Feature Flags',   desc: 'Toggle account features' },
  { to: '/admin/ferpa',            icon: FileText,        label: 'FERPA',           desc: 'Privacy & data requests' },
  { to: '/admin/terms',            icon: GraduationCap,   label: 'Terms',           desc: 'Enrollment terms' },
  { to: '/admin/grading_periods',  icon: ClipboardCheck,  label: 'Grading Periods', desc: 'Term-level grading' },
  { to: '/admin/sis_import',       icon: Upload,          label: 'SIS Import',      desc: 'Roster sync' },
  { to: '/admin/developer_keys',   icon: KeyRound,        label: 'Developer Keys',  desc: 'OAuth & LTI clients' },
  { to: '/admin/auth_providers',   icon: Shield,          label: 'Auth Providers',  desc: 'SSO configuration' },
  { to: '/admin/roles',            icon: UserCog,         label: 'Custom Roles',    desc: 'Permission overrides' },
  { to: '/admin/oneroster',        icon: RefreshCw,       label: 'OneRoster',       desc: 'OneRoster sync' },
  { to: '/settings/notifications', icon: Bell,            label: 'Notifications',   desc: 'Delivery preferences' },
  { to: '/graphiql',               icon: Code,            label: 'GraphiQL',        desc: 'Explore the GraphQL API' },
];

const AdminHomePage = () => (
  <Layout>
    <div className="p-8">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-text-primary">Admin</h1>
        <p className="text-sm text-text-secondary mt-1">
          Account-level management. Pick where to go.
        </p>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {tiles.map(({ to, icon: Icon, label, desc }) => (
          <Link
            key={to}
            to={to}
            className="group flex items-start gap-3 rounded-lg border border-border-default bg-surface-0 p-4 hover:border-brand-500 hover:bg-surface-1 transition-colors"
          >
            <div className="flex-shrink-0 rounded-md bg-brand-50 p-2 text-brand-700 group-hover:bg-brand-100">
              <Icon className="w-5 h-5" />
            </div>
            <div className="min-w-0">
              <div className="text-sm font-semibold text-text-primary">{label}</div>
              <div className="text-xs text-text-secondary mt-0.5">{desc}</div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  </Layout>
);

export default AdminHomePage;
