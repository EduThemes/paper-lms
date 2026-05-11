import React, { useState } from 'react';
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ItemPlayer from '../ItemPlayer';

// Controlled wrapper that mirrors how QuizTakePage drives ItemPlayer.
function Player({ question, initial }) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <span id={`q-${question.id}-label`}>label</span>
      <ItemPlayer question={question} value={value} onChange={setValue} />
      <textarea data-testid="value" readOnly value={JSON.stringify(value ?? null)} />
    </>
  );
}

const valueOf = () => JSON.parse(screen.getByTestId('value').value);

const baseQ = { id: 'q1', question_text: 'Question?', points_possible: 1 };

describe('ItemPlayer — multiple_answer', () => {
  it('toggles ids into the selected array', async () => {
    const question = {
      ...baseQ,
      question_type: 'multiple_answer',
      answers: JSON.stringify([
        { id: 'a', text: 'Alpha', weight: 100 },
        { id: 'b', text: 'Beta', weight: 0 },
      ]),
    };
    const user = userEvent.setup();
    render(<Player question={question} initial={[]} />);
    await user.click(screen.getByLabelText('Alpha'));
    await user.click(screen.getByLabelText('Beta'));
    expect(valueOf()).toEqual(['a', 'b']);
    await user.click(screen.getByLabelText('Alpha'));
    expect(valueOf()).toEqual(['b']);
  });
});

describe('ItemPlayer — multiple_dropdown', () => {
  it('renders an inline <select> per blank id', () => {
    const question = {
      ...baseQ,
      question_type: 'multiple_dropdown',
      question_text: 'Sky is [color] and [shape].',
      answers: JSON.stringify([
        { id: 'o1', blank_id: 'color', text: 'Blue', weight: 100 },
        { id: 'o2', blank_id: 'color', text: 'Red', weight: 0 },
        { id: 'o3', blank_id: 'shape', text: 'Round', weight: 100 },
      ]),
    };
    render(<Player question={question} initial={{}} />);
    expect(screen.getByLabelText('Blank color')).toBeInTheDocument();
    expect(screen.getByLabelText('Blank shape')).toBeInTheDocument();
  });

  it('selecting a dropdown writes into the value map', () => {
    const question = {
      ...baseQ,
      question_type: 'multiple_dropdown',
      question_text: 'Pick [x].',
      answers: JSON.stringify([
        { id: 'opt1', blank_id: 'x', text: 'One', weight: 100 },
      ]),
    };
    render(<Player question={question} initial={{}} />);
    fireEvent.change(screen.getByLabelText('Blank x'), { target: { value: 'opt1' } });
    expect(valueOf()).toEqual({ x: 'opt1' });
  });
});

describe('ItemPlayer — fill_in_the_blank', () => {
  it('records typed answer as a string', async () => {
    const question = { ...baseQ, question_type: 'fill_in_the_blank', answers: '[]' };
    const user = userEvent.setup();
    render(<Player question={question} initial="" />);
    await user.type(screen.getByPlaceholderText(/Type your answer/i), 'Paris');
    expect(valueOf()).toBe('Paris');
  });
});

describe('ItemPlayer — formula', () => {
  it('renders a numeric input', async () => {
    const question = { ...baseQ, question_type: 'formula', answers: '[]' };
    const user = userEvent.setup();
    render(<Player question={question} initial="" />);
    const input = screen.getByPlaceholderText(/Enter a numeric answer/i);
    await user.type(input, '42');
    expect(valueOf()).toBe('42');
  });
});

describe('ItemPlayer — ordering', () => {
  it('initializes order from the answer ids', () => {
    const question = {
      ...baseQ,
      question_type: 'ordering',
      answers: JSON.stringify([
        { id: 'i1', text: 'First', position: 0 },
        { id: 'i2', text: 'Second', position: 1 },
      ]),
    };
    render(<Player question={question} initial={undefined} />);
    expect(screen.getByText('First')).toBeInTheDocument();
    expect(screen.getByText('Second')).toBeInTheDocument();
  });
});

describe('ItemPlayer — categorization', () => {
  it('writes item -> bucket assignments into the value object', () => {
    const cfg = {
      id: 'a', text: '', weight: 100,
      items: [{ id: 'i1', text: 'Apple' }],
      buckets: [{ id: 'b1', label: 'Fruit', item_ids: [] }],
    };
    const question = {
      ...baseQ,
      question_type: 'categorization',
      answers: JSON.stringify([cfg]),
    };
    render(<Player question={question} initial={{}} />);
    fireEvent.change(screen.getByLabelText(/Bucket for Apple/), { target: { value: 'b1' } });
    expect(valueOf()).toEqual({ i1: 'b1' });
  });
});

describe('ItemPlayer — hot_spot', () => {
  it('renders the image when configured', () => {
    const cfg = { image_url: 'data:image/png;base64,xxx', regions: [] };
    const question = {
      ...baseQ,
      question_type: 'hot_spot',
      answers: JSON.stringify([cfg]),
    };
    render(<Player question={question} initial={null} />);
    expect(screen.getByAltText('Hot-spot question')).toBeInTheDocument();
  });
});

describe('ItemPlayer — file_upload', () => {
  it('shows a file picker', () => {
    const question = { ...baseQ, question_type: 'file_upload', answers: '[]' };
    render(<Player question={question} initial={null} />);
    expect(screen.getByText(/Choose file/)).toBeInTheDocument();
  });
});

describe('ItemPlayer — text_only', () => {
  it('renders an informational note and accepts no input', () => {
    const question = { ...baseQ, question_type: 'text_only', answers: '[]' };
    render(<Player question={question} initial={null} />);
    expect(screen.getByText(/no response required/i)).toBeInTheDocument();
  });
});

describe('ItemPlayer — legacy types still work', () => {
  it('multiple_choice renders radios and reports the chosen id', async () => {
    const question = {
      ...baseQ,
      question_type: 'multiple_choice',
      answers: JSON.stringify([
        { id: 'a', text: 'Alpha', weight: 100 },
        { id: 'b', text: 'Beta', weight: 0 },
      ]),
    };
    const user = userEvent.setup();
    render(<Player question={question} initial={null} />);
    await user.click(screen.getByLabelText('Beta'));
    expect(valueOf()).toBe('b');
  });
});
