import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BrowserRouter } from 'react-router-dom';
import LoginPage from './LoginPage';

const mockLogin = vi.fn();
const mockRegister = vi.fn();
const mockNavigate = vi.fn();

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({ login: mockLogin, register: mockRegister }),
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

function renderLoginPage() {
  const utils = render(
    <BrowserRouter>
      <LoginPage />
    </BrowserRouter>,
  );
  return utils;
}

/** Helper: find an input by its `name` attribute */
function getInput(name) {
  return document.querySelector(`input[name="${name}"]`);
}

describe('LoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test('renders login form', () => {
    renderLoginPage();

    expect(screen.getByText('Paper LMS Login')).toBeInTheDocument();
    expect(screen.getByText('Email')).toBeInTheDocument();
    expect(screen.getByText('Password')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Log In' })).toBeInTheDocument();
  });

  test('login submits credentials', async () => {
    mockLogin.mockResolvedValue({});
    const user = userEvent.setup();
    renderLoginPage();

    await user.type(getInput('email'), 'test@example.com');
    await user.type(getInput('password'), 'secret123');
    await user.click(screen.getByRole('button', { name: 'Log In' }));

    expect(mockLogin).toHaveBeenCalledWith('test@example.com', 'secret123');
  });

  test('error displayed on login failure', async () => {
    mockLogin.mockRejectedValue(new Error('Invalid credentials'));
    const user = userEvent.setup();
    renderLoginPage();

    await user.type(getInput('email'), 'bad@example.com');
    await user.type(getInput('password'), 'wrong');
    await user.click(screen.getByRole('button', { name: 'Log In' }));

    await waitFor(() => {
      expect(screen.getByText('Invalid credentials')).toBeInTheDocument();
    });

    const errorEl = screen.getByText('Invalid credentials');
    expect(errorEl).toHaveClass('text-red-600');
  });

  test('toggle to register mode', async () => {
    const user = userEvent.setup();
    renderLoginPage();

    expect(screen.queryByText('Full Name')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Register' }));

    expect(screen.getByRole('heading', { name: 'Create Account' })).toBeInTheDocument();
    expect(screen.getByText('Full Name')).toBeInTheDocument();
  });

  test('register submits name, email, and password', async () => {
    mockRegister.mockResolvedValue({});
    const user = userEvent.setup();
    renderLoginPage();

    await user.click(screen.getByRole('button', { name: 'Register' }));

    await user.type(getInput('name'), 'Jane Doe');
    await user.type(getInput('email'), 'jane@example.com');
    await user.type(getInput('password'), 'password123');
    await user.click(screen.getByRole('button', { name: 'Create Account' }));

    expect(mockRegister).toHaveBeenCalledWith('Jane Doe', 'jane@example.com', 'password123');
  });

  test('navigates to / on successful login', async () => {
    mockLogin.mockResolvedValue({});
    const user = userEvent.setup();
    renderLoginPage();

    await user.type(getInput('email'), 'test@example.com');
    await user.type(getInput('password'), 'secret123');
    await user.click(screen.getByRole('button', { name: 'Log In' }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/');
    });
  });

  test('error clears when toggling between login and register', async () => {
    mockLogin.mockRejectedValue(new Error('Login failed'));
    const user = userEvent.setup();
    renderLoginPage();

    await user.type(getInput('email'), 'bad@example.com');
    await user.type(getInput('password'), 'wrong');
    await user.click(screen.getByRole('button', { name: 'Log In' }));

    await waitFor(() => {
      expect(screen.getByText('Login failed')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: 'Register' }));

    expect(screen.queryByText('Login failed')).not.toBeInTheDocument();
  });
});
