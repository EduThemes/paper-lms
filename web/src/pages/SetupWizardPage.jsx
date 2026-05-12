import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../services/api';

const PaperLogo = () => (
  <div className="flex flex-col items-center mb-2">
    <div className="w-16 h-16 bg-brand-600 rounded-2xl flex items-center justify-center mb-3 shadow-lg">
      <svg width="36" height="36" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <path d="M4 4h16v16H4z" fill="rgba(255,255,255,0.2)" rx="2" />
        <path d="M7 8h10M7 12h7M7 16h5" stroke="#fff" strokeWidth="1.8" strokeLinecap="round" />
      </svg>
    </div>
  </div>
);

const Spinner = () => (
  <svg className="animate-spin h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" aria-hidden="true">
    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
  </svg>
);

const CheckIcon = () => (
  <svg className="w-16 h-16 text-accent-success" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2" aria-hidden="true">
    <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>
);

const StepIndicator = ({ currentStep }) => (
  <div className="flex items-center justify-center space-x-2 mb-8">
    {[1, 2, 3].map((step) => (
      <React.Fragment key={step}>
        <div
          className={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium transition-colors ${
            step === currentStep
              ? 'bg-brand-600 text-white'
              : step < currentStep
              ? 'bg-accent-success text-white'
              : 'bg-border-default text-text-tertiary'
          }`}
        >
          {step < currentStep ? (
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="3"><path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" /></svg>
          ) : (
            step
          )}
        </div>
        {step < 3 && (
          <div className={`w-12 h-0.5 ${step < currentStep ? 'bg-accent-success' : 'bg-border-default'}`} />
        )}
      </React.Fragment>
    ))}
  </div>
);

const SetupWizardPage = ({ onSetupComplete }) => {
  const navigate = useNavigate();
  const [step, setStep] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [formData, setFormData] = useState({
    admin_name: '',
    admin_email: '',
    admin_password: '',
    confirm_password: '',
    instance_name: 'Paper LMS',
  });
  const [createdUser, setCreatedUser] = useState(null);

  const handleChange = (e) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  const validateStep2 = () => {
    if (!formData.admin_name.trim()) return 'Full name is required';
    if (!formData.admin_email.trim() || !formData.admin_email.includes('@')) return 'A valid email is required';
    if (formData.admin_password.length < 8) return 'Password must be at least 8 characters';
    if (formData.admin_password !== formData.confirm_password) return 'Passwords do not match';
    return null;
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError(null);

    const validationError = validateStep2();
    if (validationError) {
      setError(validationError);
      return;
    }

    setLoading(true);
    try {
      const { data } = await api.completeSetup({
        admin_name: formData.admin_name.trim(),
        admin_email: formData.admin_email.trim(),
        admin_password: formData.admin_password,
        instance_name: formData.instance_name.trim(),
      });
      setCreatedUser(data.user);
      setStep(3);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleGoToLogin = () => {
    if (onSetupComplete) onSetupComplete();
    navigate('/login');
  };

  return (
    <div className="min-h-screen bg-surface-0 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <div className="bg-surface-raised rounded-2xl shadow-xl p-8">
          <PaperLogo />
          <StepIndicator currentStep={step} />

          {/* Step 1: Welcome */}
          {step === 1 && (
            <div className="text-center">
              <h2 className="text-2xl font-bold text-text-primary mb-2">Welcome to Paper LMS</h2>
              <p className="text-text-secondary mb-6">
                Let's set up your instance in a few quick steps. You'll create an administrator account and name your instance.
              </p>
              <button
                onClick={() => setStep(2)}
                className="w-full py-3 bg-brand-600 text-white rounded-lg font-medium hover:bg-brand-700 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
              >
                Get Started
              </button>
            </div>
          )}

          {/* Step 2: Create Admin Account */}
          {step === 2 && (
            <div>
              <h2 className="text-xl font-bold text-text-primary mb-1 text-center">Create Admin Account</h2>
              <p className="text-sm text-text-tertiary mb-5 text-center">This will be the primary administrator for your instance.</p>

              {error && (
                <div className="mb-4 p-3 bg-accent-danger/10 border border-accent-danger/30 rounded-lg text-accent-danger text-sm">
                  {error}
                </div>
              )}

              <form onSubmit={handleSubmit} className="space-y-4">
                <div>
                  <label htmlFor="admin_name" className="block text-sm font-medium text-text-secondary mb-1">Full Name</label>
                  <input
                    id="admin_name"
                    name="admin_name"
                    type="text"
                    required
                    value={formData.admin_name}
                    onChange={handleChange}
                    className="w-full px-3 py-2 border border-border-strong rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                    placeholder="Jane Smith"
                  />
                </div>
                <div>
                  <label htmlFor="admin_email" className="block text-sm font-medium text-text-secondary mb-1">Email</label>
                  <input
                    id="admin_email"
                    name="admin_email"
                    type="email"
                    required
                    value={formData.admin_email}
                    onChange={handleChange}
                    className="w-full px-3 py-2 border border-border-strong rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                    placeholder="admin@school.edu"
                  />
                </div>
                <div>
                  <label htmlFor="admin_password" className="block text-sm font-medium text-text-secondary mb-1">Password</label>
                  <input
                    id="admin_password"
                    name="admin_password"
                    type="password"
                    required
                    minLength={8}
                    value={formData.admin_password}
                    onChange={handleChange}
                    className="w-full px-3 py-2 border border-border-strong rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                    placeholder="Minimum 8 characters"
                  />
                </div>
                <div>
                  <label htmlFor="confirm_password" className="block text-sm font-medium text-text-secondary mb-1">Confirm Password</label>
                  <input
                    id="confirm_password"
                    name="confirm_password"
                    type="password"
                    required
                    minLength={8}
                    value={formData.confirm_password}
                    onChange={handleChange}
                    className="w-full px-3 py-2 border border-border-strong rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                    placeholder="Re-enter password"
                  />
                </div>
                <div>
                  <label htmlFor="instance_name" className="block text-sm font-medium text-text-secondary mb-1">Instance Name</label>
                  <input
                    id="instance_name"
                    name="instance_name"
                    type="text"
                    value={formData.instance_name}
                    onChange={handleChange}
                    className="w-full px-3 py-2 border border-border-strong rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                    placeholder="Paper LMS"
                  />
                  <p className="text-xs text-text-disabled mt-1">The name displayed across your LMS instance.</p>
                </div>

                <div className="flex space-x-3 pt-2">
                  <button
                    type="button"
                    onClick={() => { setStep(1); setError(null); }}
                    className="flex-1 py-3 border border-border-strong text-text-secondary rounded-lg font-medium hover:bg-surface-1 transition-colors focus:outline-none focus:ring-2 focus:ring-gray-300"
                  >
                    Back
                  </button>
                  <button
                    type="submit"
                    disabled={loading}
                    className="flex-1 py-3 bg-brand-600 text-white rounded-lg font-medium hover:bg-brand-700 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center"
                  >
                    {loading ? <Spinner /> : 'Complete Setup'}
                  </button>
                </div>
              </form>
            </div>
          )}

          {/* Step 3: Complete */}
          {step === 3 && (
            <div className="text-center">
              <div className="flex justify-center mb-4">
                <CheckIcon />
              </div>
              <h2 className="text-2xl font-bold text-text-primary mb-2">Setup Complete!</h2>
              <p className="text-text-secondary mb-6">Your Paper LMS instance is ready to use.</p>

              <div className="bg-surface-1 rounded-lg p-4 mb-6 text-left">
                <div className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-text-tertiary">Admin Email</span>
                    <span className="font-medium text-text-primary">{createdUser?.email || formData.admin_email}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-text-tertiary">Instance Name</span>
                    <span className="font-medium text-text-primary">{formData.instance_name || 'Paper LMS'}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-text-tertiary">Role</span>
                    <span className="font-medium text-text-primary">Administrator</span>
                  </div>
                </div>
              </div>

              <button
                onClick={handleGoToLogin}
                className="w-full py-3 bg-brand-600 text-white rounded-lg font-medium hover:bg-brand-700 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
              >
                Go to Login
              </button>
            </div>
          )}
        </div>

        <p className="text-center text-xs text-text-disabled mt-6">Paper LMS &mdash; Open-source learning management</p>
      </div>
    </div>
  );
};

export default SetupWizardPage;
