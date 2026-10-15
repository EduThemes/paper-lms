/* global describe, test, expect */
import { render, screen } from '@testing-library/react';
import CourseHero from '../CourseHero';

const look = {
  version: 1,
  theme_id: 'tide-pool',
  palette_id: 'default',
  colors: { primary: '#12355B', on_primary: '#ffffff', accent: '#f4a261' },
  dark_colors: { primary: '#9cc4ef', on_primary: '#0b1b2e' },
  banner: {
    src: '/design-assets/themes/tide-pool/banner.webp',
    src_2x: '/design-assets/themes/tide-pool/banner@2x.webp',
    alt: '',
  },
};

describe('CourseHero', () => {
  test('falls back to the generated gradient without a look', () => {
    render(<CourseHero course={{ name: 'Biology', course_code: 'BIO 101' }} courseId="7" progress={40} />);
    const hero = screen.getByTestId('course-hero');
    expect(hero).toHaveClass('text-white');
    expect(hero).not.toHaveClass('course-look');
    expect(screen.getByTestId('course-hero-gradient').style.background).toContain('linear-gradient');
    expect(screen.queryByTestId('course-hero-banner')).toBeNull();
    expect(screen.getByRole('heading', { level: 1, name: 'Biology' })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Course progress: 40 percent' })).toBeInTheDocument();
  });

  test('uses the look: banner art with a 2x source, colours as custom properties', () => {
    render(<CourseHero course={{ name: 'Biology', look }} courseId="7" progress={0} />);
    const hero = screen.getByTestId('course-hero');
    expect(hero).toHaveClass('course-look');
    expect(hero).toHaveAttribute('data-look', 'tide-pool');
    expect(hero.style.getPropertyValue('--look-primary')).toBe('#12355b');
    expect(hero.style.getPropertyValue('--look-primary-rgb')).toBe('18 53 91');
    expect(hero.style.getPropertyValue('--look-on-primary')).toBe('#ffffff');
    expect(hero.style.getPropertyValue('--look-primary-dark')).toBe('#9cc4ef');
    expect(hero.style.getPropertyValue('--look-on-primary-dark')).toBe('#0b1b2e');
    const banner = screen.getByTestId('course-hero-banner');
    expect(banner).toHaveAttribute('src', look.banner.src);
    expect(banner).toHaveAttribute('srcset', `${look.banner.src} 1x, ${look.banner.src_2x} 2x`);
    expect(banner).toHaveAttribute('alt', '');
    expect(screen.queryByTestId('course-hero-gradient')).toBeNull();
    // Text sits on a scrim in the look's colour.
    expect(hero.querySelector('.course-look-scrim')).not.toBeNull();
    expect(hero.querySelector('.course-look-text h1')).toHaveTextContent('Biology');
  });

  test('a look without banner art still colours the header', () => {
    render(<CourseHero course={{ name: 'Art', look: { ...look, banner: null } }} courseId="7" />);
    expect(screen.getByTestId('course-hero')).toHaveClass('course-look');
    expect(screen.queryByTestId('course-hero-banner')).toBeNull();
  });

  test('ignores an unusable look (bad colour) and a banner from elsewhere', () => {
    const { unmount } = render(
      <CourseHero course={{ name: 'Art', look: { ...look, colors: { primary: 'red' } } }} courseId="7" />,
    );
    expect(screen.getByTestId('course-hero-gradient')).toBeInTheDocument();
    unmount();

    render(
      <CourseHero
        course={{ name: 'Art', look: { ...look, banner: { src: 'https://evil.example/x.png' } } }}
        courseId="7"
      />,
    );
    expect(screen.getByTestId('course-hero')).toHaveClass('course-look');
    expect(screen.queryByTestId('course-hero-banner')).toBeNull();
  });
});
