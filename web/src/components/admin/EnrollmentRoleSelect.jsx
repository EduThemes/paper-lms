import React, { useEffect, useState } from 'react';
import { adminAccountsApi } from '../../services/api_admin_accounts';
import { roleChoiceValue } from '../../utils/adminAccounts';

const BASE_ROLES = [
  ['StudentEnrollment', 'Student'],
  ['TeacherEnrollment', 'Teacher'],
  ['TaEnrollment', 'TA'],
  ['ObserverEnrollment', 'Observer'],
];

// Role picker for the course Add People form (G102): the base roles plus
// the account's custom course roles, each on top of its base type. The
// value is "Type" or "Type|roleId" (parseRoleChoice reads it back).
export default function EnrollmentRoleSelect({ courseId, value, onChange, id = 'add-person-role', className = '' }) {
  const [customRoles, setCustomRoles] = useState([]);

  useEffect(() => {
    let alive = true;
    if (!courseId) return undefined;
    Promise.resolve()
      .then(() => adminAccountsApi.getAssignableRoles(courseId))
      .then((roles) => { if (alive) setCustomRoles(Array.isArray(roles) ? roles.filter((r) => r.enrollment_type) : []); })
      .catch(() => { if (alive) setCustomRoles([]); });
    return () => { alive = false; };
  }, [courseId]);

  return (
    <select id={id} value={value} onChange={(e) => onChange(e.target.value)} className={className}>
      {BASE_ROLES.map(([type, label]) => (
        <option key={type} value={type}>{label}</option>
      ))}
      {customRoles.length > 0 && (
        <optgroup label="Custom roles">
          {customRoles.map((r) => (
            <option key={r.id} value={roleChoiceValue(r.enrollment_type, r.id)}>
              {r.label || r.name}
            </option>
          ))}
        </optgroup>
      )}
    </select>
  );
}
