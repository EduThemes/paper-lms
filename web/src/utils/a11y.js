// clickableProps makes a non-button element that already reacts to clicks
// operable from the keyboard too (G159, WCAG 2.1.1): it becomes focusable,
// is exposed as a button (unless a role is given — e.g. null for a table
// row, which must keep its row semantics) and Enter / Space activate it.
//
//   <div {...clickableProps(() => toggle(id))} className="...">
export function clickableProps(onActivate, { role = 'button' } = {}) {
  const props = {
    tabIndex: 0,
    onClick: onActivate,
    onKeyDown: (e) => {
      if (e.target !== e.currentTarget) return; // let inner controls handle their keys
      if (e.key === 'Enter' || e.key === ' ' || e.key === 'Spacebar') {
        e.preventDefault();
        onActivate(e);
      }
    },
  };
  if (role) props.role = role;
  return props;
}
