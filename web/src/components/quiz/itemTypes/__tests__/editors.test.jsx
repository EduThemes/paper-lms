import React, { useState } from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MultipleAnswerEditor from '../MultipleAnswerEditor';
import MultipleDropdownEditor, { extractBlankIds } from '../MultipleDropdownEditor';
import FillInBlankEditor from '../FillInBlankEditor';
import FormulaEditor from '../FormulaEditor';
import FileUploadEditor from '../FileUploadEditor';
import OrderingEditor from '../OrderingEditor';
import CategorizationEditor from '../CategorizationEditor';
import HotSpotEditor from '../HotSpotEditor';
import TextOnlyEditor from '../TextOnlyEditor';
import { defaultAnswersForType } from '../types';

// Tiny controlled wrapper to mimic the QuestionEditor's `answers` state plumbing.
function Harness({ Editor, type, extraProps = {} }) {
  const [answers, setAnswers] = useState(defaultAnswersForType(type));
  return (
    <>
      <Editor answers={answers} onChange={setAnswers} {...extraProps} />
      <textarea data-testid="state" readOnly value={JSON.stringify(answers)} />
    </>
  );
}

const stateOf = () => JSON.parse(screen.getByTestId('state').value);

describe('MultipleAnswerEditor', () => {
  it('renders one row per answer with a toggle button', () => {
    render(<Harness Editor={MultipleAnswerEditor} type="multiple_answer" />);
    const rows = screen.getAllByRole('button', { name: /Remove answer|Add Answer/i });
    expect(rows.length).toBeGreaterThan(0);
  });
  it('toggles correctness when the checkbox is clicked', async () => {
    render(<Harness Editor={MultipleAnswerEditor} type="multiple_answer" />);
    const initial = stateOf().filter(a => a.weight > 0).length;
    const toggles = screen.getAllByRole('button', { pressed: true });
    fireEvent.click(toggles[0]);
    const after = stateOf().filter(a => a.weight > 0).length;
    expect(after).toBe(initial - 1);
  });
  it('add/remove answer mutates the list', async () => {
    render(<Harness Editor={MultipleAnswerEditor} type="multiple_answer" />);
    const startLen = stateOf().length;
    fireEvent.click(screen.getByText(/Add Answer/i));
    expect(stateOf().length).toBe(startLen + 1);
  });
});

describe('MultipleDropdownEditor', () => {
  it('extractBlankIds picks up [id] tokens once per unique id', () => {
    expect(extractBlankIds('Pick [color] and [shape] and [color] again')).toEqual(['color', 'shape']);
    expect(extractBlankIds('')).toEqual([]);
  });
  it('renders one option group per blank in the question text', () => {
    render(<Harness Editor={MultipleDropdownEditor} type="multiple_dropdown"
                    extraProps={{ questionText: 'Fill [blank1] and [blank2]' }} />);
    expect(screen.getByText('[blank1]')).toBeInTheDocument();
    expect(screen.getByText('[blank2]')).toBeInTheDocument();
  });
  it('warns when no blank tokens are present', () => {
    render(<Harness Editor={MultipleDropdownEditor} type="multiple_dropdown"
                    extraProps={{ questionText: 'no tokens' }} />);
    expect(screen.getByText(/No blank placeholders/i)).toBeInTheDocument();
  });
});

describe('FillInBlankEditor', () => {
  it('renders the accepted-answer rows', () => {
    render(<Harness Editor={FillInBlankEditor} type="fill_in_the_blank" />);
    expect(screen.getByPlaceholderText(/Accepted answer/i)).toBeInTheDocument();
  });
  it('captures input into state', async () => {
    const user = userEvent.setup();
    render(<Harness Editor={FillInBlankEditor} type="fill_in_the_blank" />);
    await user.type(screen.getByPlaceholderText(/Accepted answer/i), 'Paris');
    expect(stateOf()[0].text).toBe('Paris');
  });
});

describe('FormulaEditor', () => {
  it('renders formula input and tolerance', () => {
    render(<Harness Editor={FormulaEditor} type="formula" />);
    expect(screen.getByPlaceholderText(/x \* 2 \+ y/)).toBeInTheDocument();
  });
  it('add variable updates state', async () => {
    render(<Harness Editor={FormulaEditor} type="formula" />);
    fireEvent.click(screen.getByText(/Add Variable/i));
    expect(stateOf()[0].variables.length).toBe(1);
  });
});

describe('FileUploadEditor', () => {
  it('renders the manual-grading note', () => {
    render(<FileUploadEditor />);
    expect(screen.getByText(/Manual grading required/i)).toBeInTheDocument();
  });
});

describe('OrderingEditor', () => {
  it('renders all items in their listed order', () => {
    render(<Harness Editor={OrderingEditor} type="ordering" />);
    expect(screen.getByDisplayValue('First item')).toBeInTheDocument();
    expect(screen.getByDisplayValue('Second item')).toBeInTheDocument();
  });
  it('adds a new ordering item with sequential position', () => {
    render(<Harness Editor={OrderingEditor} type="ordering" />);
    fireEvent.click(screen.getByText(/Add Item/i));
    const items = stateOf();
    expect(items[items.length - 1].position).toBe(items.length - 1);
  });
});

describe('CategorizationEditor', () => {
  it('renders items and buckets columns', () => {
    render(<Harness Editor={CategorizationEditor} type="categorization" />);
    expect(screen.getByText(/Buckets/)).toBeInTheDocument();
    expect(screen.getAllByText(/Items/)[0]).toBeInTheDocument();
  });
  it('assigning an item to a bucket records the relationship', () => {
    render(<Harness Editor={CategorizationEditor} type="categorization" />);
    const selects = screen.getAllByRole('combobox');
    // The first item-to-bucket select; pick the first non-empty option.
    const target = selects[0];
    const opts = Array.from(target.options).map(o => o.value).filter(Boolean);
    fireEvent.change(target, { target: { value: opts[0] } });
    const cfg = stateOf()[0];
    const bucket = cfg.buckets.find(b => b.id === opts[0]);
    expect(bucket.item_ids.length).toBeGreaterThan(0);
  });
});

describe('HotSpotEditor', () => {
  it('shows the upload prompt before an image is provided', () => {
    render(<Harness Editor={HotSpotEditor} type="hot_spot" extraProps={{ courseId: 1 }} />);
    expect(screen.getByText(/Upload an image to begin/i)).toBeInTheDocument();
  });
});

describe('TextOnlyEditor', () => {
  it('renders the not-graded notice', () => {
    render(<TextOnlyEditor />);
    expect(screen.getByText(/Passage \/ Instructions only/i)).toBeInTheDocument();
  });
});
