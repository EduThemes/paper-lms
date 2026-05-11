import React from 'react';

const presetColors = [
  '#0374B5', '#2D9CDB', '#27AE60', '#219653',
  '#F2994A', '#EB5757', '#9B51E0', '#BB6BD9',
  '#F2C94C', '#4F4F4F', '#333333', '#828282',
];

const ColorPicker = ({ value, onChange }) => {
  return (
    <div className="flex items-center gap-2 flex-wrap">
      {presetColors.map(color => (
        <button key={color} type="button" onClick={() => onChange(color)}
          className={`w-8 h-8 rounded-full border-2 transition-transform hover:scale-110 ${value === color ? 'border-gray-900 scale-110' : 'border-transparent'}`}
          style={{ backgroundColor: color }} title={color} />
      ))}
      <input type="color" value={value || '#0374B5'} onChange={e => onChange(e.target.value)}
        className="w-8 h-8 rounded cursor-pointer" title="Custom color" />
    </div>
  );
};

export default ColorPicker;
