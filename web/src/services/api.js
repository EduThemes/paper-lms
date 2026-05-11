const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

function getCSRFToken() {
  const match = document.cookie.match(/(?:^|;\s*)paper_csrf=([^;]*)/);
  return match ? match[1] : '';
}

const getHeaders = () => {
  return {
    'Content-Type': 'application/json',
    'X-CSRF-Token': getCSRFToken(),
  };
};

function parseLinkHeader(header) {
  if (!header) return {};
  const links = {};
  const parts = header.split(',');
  for (const part of parts) {
    const match = part.match(/<([^>]+)>;\s*rel="([^"]+)"/);
    if (match) {
      links[match[2]] = match[1];
    }
  }
  return links;
}

async function request(path, options = {}) {
  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    credentials: 'include',
    headers: { ...getHeaders(), ...options.headers },
  });

  if (!response.ok) {
    if (response.status === 401) {
      window.dispatchEvent(new Event('auth:session-expired'));
    }
    const body = await response.json().catch(() => ({}));
    const message = body.errors?.[0]?.message || `Request failed: ${response.status}`;
    throw new Error(message);
  }

  const data = await response.json();
  const linkHeader = response.headers.get('Link');
  const pagination = parseLinkHeader(linkHeader);

  return { data, pagination };
}

async function requestRaw(path, options = {}) {
  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    credentials: 'include',
    headers: { ...getHeaders(), ...options.headers },
  });
  if (!response.ok) {
    if (response.status === 401) {
      window.dispatchEvent(new Event('auth:session-expired'));
    }
    const body = await response.json().catch(() => ({}));
    const message = body.errors?.[0]?.message || `Request failed: ${response.status}`;
    throw new Error(message);
  }
  return response;
}

async function uploadFile(path, formData) {
  const response = await fetch(`${API_URL}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'X-CSRF-Token': getCSRFToken() },
    body: formData,
  });
  if (!response.ok) {
    if (response.status === 401) {
      window.dispatchEvent(new Event('auth:session-expired'));
    }
    const body = await response.json().catch(() => ({}));
    const message = body.errors?.[0]?.message || `Request failed: ${response.status}`;
    throw new Error(message);
  }
  return response.json();
}

export const api = {
  // Generic request method for pages that need direct API access
  request,

  // Setup
  getSetupStatus: () => request('/setup/status'),
  completeSetup: (data) => request('/setup/complete', {
    method: 'POST',
    body: JSON.stringify(data),
  }),

  // Auth
  login: async (email, password) => {
    const { data } = await request('/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
    return data;
  },

  requestPasswordReset: async (email) => {
    const { data } = await request('/password/reset', {
      method: 'POST',
      body: JSON.stringify({ email }),
    });
    return data;
  },

  resetPassword: async (token, newPassword) => {
    const { data } = await request('/password/reset/confirm', {
      method: 'POST',
      body: JSON.stringify({ token, new_password: newPassword }),
    });
    return data;
  },

  register: async (name, email, password) => {
    const { data } = await request('/register', {
      method: 'POST',
      body: JSON.stringify({ name, email, password }),
    });
    return data;
  },

  logout: async () => {
    const { data } = await request('/logout', { method: 'POST' });
    return data;
  },

  // Users
  getSelf: async () => {
    const { data } = await request('/users/self');
    return data;
  },

  getUser: async (id) => {
    const { data } = await request(`/users/${id}`);
    return data;
  },

  searchUsers: async (searchTerm, page = 1, perPage = 10) => {
    return request(`/users?search_term=${encodeURIComponent(searchTerm)}&page=${page}&per_page=${perPage}`);
  },

  listUsers: async (page = 1, perPage = 25) => {
    return request(`/users?page=${page}&per_page=${perPage}`);
  },

  updateUserRole: async (userId, role) => {
    const { data } = await request(`/users/${userId}/role`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    });
    return data;
  },

  // Admin: account settings
  getAccount: async (id = 1) => {
    const { data } = await request(`/accounts/${id}`);
    return data;
  },
  updateAccount: async (id, payload) => {
    const { data } = await request(`/accounts/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
    return data;
  },

  // Masquerade (act-as-user)
  startMasquerade: async (userId) => {
    const { data } = await request(`/users/${userId}/masquerade`, { method: 'POST' });
    return data;
  },
  endMasquerade: async () => {
    const { data } = await request('/masquerade', { method: 'DELETE' });
    return data;
  },

  // Courses
  getCourses: async (page = 1, perPage = 10) => {
    return request(`/courses?page=${page}&per_page=${perPage}`);
  },

  getAllCourses: async (page = 1, perPage = 10) => {
    return request(`/courses?scope=all&page=${page}&per_page=${perPage}`);
  },

  getCourse: async (id) => {
    const { data } = await request(`/courses/${id}`);
    return data;
  },

  createCourse: async (course) => {
    const { data } = await request('/courses', {
      method: 'POST',
      body: JSON.stringify({ course }),
    });
    return data;
  },

  updateCourse: async (id, course) => {
    const { data } = await request(`/courses/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ course }),
    });
    return data;
  },

  deleteCourse: async (id) => {
    const { data } = await request(`/courses/${id}`, { method: 'DELETE' });
    return data;
  },

  // Sections
  getSections: async (courseId, page = 1, perPage = 10) => {
    return request(`/courses/${courseId}/sections?page=${page}&per_page=${perPage}`);
  },

  createSection: async (courseId, courseSection) => {
    const { data } = await request(`/courses/${courseId}/sections`, {
      method: 'POST',
      body: JSON.stringify({ course_section: courseSection }),
    });
    return data;
  },

  // Enrollments
  getEnrollments: async (courseId, page = 1, perPage = 10) => {
    return request(`/courses/${courseId}/enrollments?page=${page}&per_page=${perPage}`);
  },

  createEnrollment: async (courseId, enrollment) => {
    const { data } = await request(`/courses/${courseId}/enrollments`, {
      method: 'POST',
      body: JSON.stringify({ enrollment }),
    });
    return data;
  },

  // Modules
  getModules: async (courseId, page = 1, perPage = 10, includeItems = true) => {
    const include = includeItems ? '&include[]=items' : '';
    return request(`/courses/${courseId}/modules?page=${page}&per_page=${perPage}${include}`);
  },

  getModule: async (courseId, moduleId) => {
    const { data } = await request(`/courses/${courseId}/modules/${moduleId}`);
    return data;
  },

  createModule: async (courseId, module) => {
    const { data } = await request(`/courses/${courseId}/modules`, {
      method: 'POST',
      body: JSON.stringify({ module }),
    });
    return data;
  },

  updateModule: async (courseId, moduleId, module) => {
    const { data } = await request(`/courses/${courseId}/modules/${moduleId}`, {
      method: 'PUT',
      body: JSON.stringify({ module }),
    });
    return data;
  },

  deleteModule: async (courseId, moduleId) => {
    const { data } = await request(`/courses/${courseId}/modules/${moduleId}`, { method: 'DELETE' });
    return data;
  },

  // Module Items
  getModuleItems: async (courseId, moduleId, page = 1, perPage = 10) => {
    return request(`/courses/${courseId}/modules/${moduleId}/items?page=${page}&per_page=${perPage}`);
  },

  createModuleItem: async (courseId, moduleId, moduleItem) => {
    const { data } = await request(`/courses/${courseId}/modules/${moduleId}/items`, {
      method: 'POST',
      body: JSON.stringify({ module_item: moduleItem }),
    });
    return data;
  },

  updateModuleItem: async (courseId, moduleId, itemId, moduleItem) => {
    const { data } = await request(`/courses/${courseId}/modules/${moduleId}/items/${itemId}`, {
      method: 'PUT',
      body: JSON.stringify({ module_item: moduleItem }),
    });
    return data;
  },

  deleteModuleItem: async (courseId, moduleId, itemId) => {
    const { data } = await request(`/courses/${courseId}/modules/${moduleId}/items/${itemId}`, {
      method: 'DELETE',
    });
    return data;
  },

  reorderModules: async (courseId, order) => {
    const { data } = await request(`/courses/${courseId}/modules/reorder`, {
      method: 'POST',
      body: JSON.stringify({ order }),
    });
    return data;
  },

  reorderModuleItems: async (courseId, moduleId, order) => {
    const { data } = await request(`/courses/${courseId}/modules/${moduleId}/items/reorder`, {
      method: 'POST',
      body: JSON.stringify({ order }),
    });
    return data;
  },

  moveModuleItem: async (courseId, moduleId, itemId, targetModuleId, position) => {
    const { data } = await request(`/courses/${courseId}/modules/${moduleId}/items/${itemId}/move`, {
      method: 'POST',
      body: JSON.stringify({ module_id: targetModuleId, position }),
    });
    return data;
  },

  // Pages
  getPages: async (courseId, page = 1, perPage = 10) => {
    return request(`/courses/${courseId}/pages?page=${page}&per_page=${perPage}`);
  },

  getPage: async (courseId, urlOrId) => {
    const { data } = await request(`/courses/${courseId}/pages/${urlOrId}`);
    return data;
  },

  createPage: async (courseId, wikiPage) => {
    const { data } = await request(`/courses/${courseId}/pages`, {
      method: 'POST',
      body: JSON.stringify({ wiki_page: wikiPage }),
    });
    return data;
  },

  updatePage: async (courseId, urlOrId, wikiPage) => {
    const { data } = await request(`/courses/${courseId}/pages/${urlOrId}`, {
      method: 'PUT',
      body: JSON.stringify({ wiki_page: wikiPage }),
    });
    return data;
  },

  deletePage: async (courseId, urlOrId) => {
    const { data } = await request(`/courses/${courseId}/pages/${urlOrId}`, { method: 'DELETE' });
    return data;
  },

  getPublicPage: async (courseId, slug) => {
    const { data } = await request(`/courses/${courseId}/p/${slug}`);
    return data;
  },

  // Assignments
  getAssignments: async (courseId, page = 1, perPage = 10) => {
    return request(`/courses/${courseId}/assignments?page=${page}&per_page=${perPage}`);
  },

  getAssignment: async (courseId, assignmentId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}`);
    return data;
  },

  createAssignment: async (courseId, assignment) => {
    const { data } = await request(`/courses/${courseId}/assignments`, {
      method: 'POST',
      body: JSON.stringify({ assignment }),
    });
    return data;
  },

  updateAssignment: async (courseId, assignmentId, assignment) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}`, {
      method: 'PUT',
      body: JSON.stringify({ assignment }),
    });
    return data;
  },

  deleteAssignment: async (courseId, assignmentId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}`, { method: 'DELETE' });
    return data;
  },

  // Assignment Groups
  getAssignmentGroups: async (courseId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/assignment_groups?page=${page}&per_page=${perPage}`);
  },
  createAssignmentGroup: async (courseId, assignmentGroup) => {
    const { data } = await request(`/courses/${courseId}/assignment_groups`, {
      method: 'POST', body: JSON.stringify({ assignment_group: assignmentGroup }),
    });
    return data;
  },

  // Submissions
  getSubmissions: async (courseId, assignmentId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/assignments/${assignmentId}/submissions?page=${page}&per_page=${perPage}`);
  },
  getSubmission: async (courseId, assignmentId, userId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/submissions/${userId}`);
    return data;
  },
  createSubmission: async (courseId, assignmentId, submission) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/submissions`, {
      method: 'POST', body: JSON.stringify({ submission }),
    });
    return data;
  },
  gradeSubmission: async (courseId, assignmentId, userId, submission) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/submissions/${userId}`, {
      method: 'PUT', body: JSON.stringify({ submission }),
    });
    return data;
  },

  bulkGrade: async (courseId, gradeData) => {
    const { data } = await request(`/courses/${courseId}/submissions/bulk_grade`, {
      method: 'POST', body: JSON.stringify({ grade_data: gradeData }),
    });
    return data;
  },

  postGrades: async (courseId, assignmentId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/post_grades`, {
      method: 'POST',
    });
    return data;
  },
  hideGrades: async (courseId, assignmentId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/hide_grades`, {
      method: 'POST',
    });
    return data;
  },

  // Submission Comments
  getSubmissionComments: async (courseId, assignmentId, userId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/submissions/${userId}/comments`);
    return data;
  },
  createSubmissionComment: async (courseId, assignmentId, userId, comment) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/submissions/${userId}/comments`, {
      method: 'POST', body: JSON.stringify({ comment }),
    });
    return data;
  },

  // Course Submissions (bulk)
  getCourseSubmissions: async (courseId, page = 1, perPage = 10000, userId = null) => {
    const userFilter = userId ? `&user_id=${userId}` : '';
    return request(`/courses/${courseId}/submissions?page=${page}&per_page=${perPage}${userFilter}`);
  },

  // Gradebook
  getGradebook: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/gradebook`);
    return data;
  },
  getStudentGrade: async (courseId, studentId) => {
    const { data } = await request(`/courses/${courseId}/students/${studentId}/grade`);
    return data;
  },

  // Grading Standards
  getGradingStandards: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/grading_standards`);
    return data;
  },
  createGradingStandard: async (courseId, title, data) => {
    const { data: result } = await request(`/courses/${courseId}/grading_standards`, {
      method: 'POST', body: JSON.stringify({ grading_standard: { title, data } }),
    });
    return result;
  },
  updateGradingStandard: async (courseId, id, title, data) => {
    const { data: result } = await request(`/courses/${courseId}/grading_standards/${id}`, {
      method: 'PUT', body: JSON.stringify({ grading_standard: { title, data } }),
    });
    return result;
  },
  deleteGradingStandard: async (courseId, id) => {
    const { data: result } = await request(`/courses/${courseId}/grading_standards/${id}`, { method: 'DELETE' });
    return result;
  },

  // Developer Keys
  getDeveloperKeys: async (accountId = 1, page = 1, perPage = 10) => {
    return request(`/accounts/${accountId}/developer_keys?page=${page}&per_page=${perPage}`);
  },
  createDeveloperKey: async (accountId = 1, developerKey) => {
    const { data } = await request(`/accounts/${accountId}/developer_keys`, {
      method: 'POST', body: JSON.stringify({ developer_key: developerKey }),
    });
    return data;
  },
  updateDeveloperKey: async (accountId, keyId, developerKey) => {
    const { data } = await request(`/accounts/${accountId}/developer_keys/${keyId}`, {
      method: 'PUT', body: JSON.stringify({ developer_key: developerKey }),
    });
    return data;
  },
  deleteDeveloperKey: async (accountId, keyId) => {
    const { data } = await request(`/accounts/${accountId}/developer_keys/${keyId}`, { method: 'DELETE' });
    return data;
  },

  // Personal Access Tokens
  getAccessTokens: async (userId, page = 1, perPage = 10) => {
    return request(`/users/${userId}/tokens?page=${page}&per_page=${perPage}`);
  },
  createAccessToken: async (userId, token) => {
    const { data } = await request(`/users/${userId}/tokens`, {
      method: 'POST', body: JSON.stringify({ token }),
    });
    return data;
  },
  deleteAccessToken: async (userId, tokenId) => {
    const { data } = await request(`/users/${userId}/tokens/${tokenId}`, { method: 'DELETE' });
    return data;
  },

  // External Tools
  getExternalTools: async (courseId, page = 1, perPage = 10) => {
    return request(`/courses/${courseId}/external_tools?page=${page}&per_page=${perPage}`);
  },
  createExternalTool: async (courseId, externalTool) => {
    const { data } = await request(`/courses/${courseId}/external_tools`, {
      method: 'POST', body: JSON.stringify({ external_tool: externalTool }),
    });
    return data;
  },
  updateExternalTool: async (courseId, toolId, externalTool) => {
    const { data } = await request(`/courses/${courseId}/external_tools/${toolId}`, {
      method: 'PUT', body: JSON.stringify({ external_tool: externalTool }),
    });
    return data;
  },
  deleteExternalTool: async (courseId, toolId) => {
    const { data } = await request(`/courses/${courseId}/external_tools/${toolId}`, { method: 'DELETE' });
    return data;
  },

  // Discussion Topics
  getDiscussionTopics: async (courseId, page = 1, perPage = 20) => {
    return request(`/courses/${courseId}/discussion_topics?page=${page}&per_page=${perPage}`);
  },
  getDiscussionTopic: async (courseId, topicId) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}`);
    return data;
  },
  createDiscussionTopic: async (courseId, topic) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics`, {
      method: 'POST', body: JSON.stringify({ discussion_topic: topic }),
    });
    return data;
  },
  updateDiscussionTopic: async (courseId, topicId, topic) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}`, {
      method: 'PUT', body: JSON.stringify({ discussion_topic: topic }),
    });
    return data;
  },
  deleteDiscussionTopic: async (courseId, topicId) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}`, { method: 'DELETE' });
    return data;
  },
  getDiscussionTopicView: async (courseId, topicId) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/view`);
    return data;
  },

  // Discussion Entries
  getDiscussionEntries: async (courseId, topicId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/discussion_topics/${topicId}/entries?page=${page}&per_page=${perPage}`);
  },
  createDiscussionEntry: async (courseId, topicId, message) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/entries`, {
      method: 'POST', body: JSON.stringify({ message }),
    });
    return data;
  },
  updateDiscussionEntry: async (courseId, topicId, entryId, message) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/entries/${entryId}`, {
      method: 'PUT', body: JSON.stringify({ message }),
    });
    return data;
  },
  deleteDiscussionEntry: async (courseId, topicId, entryId) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/entries/${entryId}`, { method: 'DELETE' });
    return data;
  },
  getDiscussionEntryReplies: async (courseId, topicId, entryId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/discussion_topics/${topicId}/entries/${entryId}/replies?page=${page}&per_page=${perPage}`);
  },
  createDiscussionReply: async (courseId, topicId, entryId, message) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/entries/${entryId}/replies`, {
      method: 'POST', body: JSON.stringify({ message }),
    });
    return data;
  },
  rateDiscussionEntry: async (courseId, topicId, entryId, rating) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/entries/${entryId}/rating`, {
      method: 'POST', body: JSON.stringify({ rating }),
    });
    return data;
  },

  // Files
  getCourseFiles: async (courseId, page = 1, perPage = 20) => {
    return request(`/courses/${courseId}/files?page=${page}&per_page=${perPage}`);
  },
  uploadCourseFile: async (courseId, file) => {
    const formData = new FormData();
    formData.append('file', file);
    return uploadFile(`/courses/${courseId}/files`, formData);
  },
  getFile: async (courseId, fileId) => {
    const { data } = await request(`/courses/${courseId}/files/${fileId}`);
    return data;
  },
  deleteFile: async (courseId, fileId) => {
    const { data } = await request(`/courses/${courseId}/files/${fileId}`, { method: 'DELETE' });
    return data;
  },
  getFileDownloadUrl: (fileId) => `${API_URL}/files/${fileId}/download`,
  uploadCourseFileWithProgress: (courseId, file, onProgress) => {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      const formData = new FormData();
      formData.append('file', file);
      xhr.open('POST', `${API_URL}/courses/${courseId}/files`);
      xhr.withCredentials = true;
      xhr.setRequestHeader('X-CSRF-Token', getCSRFToken());
      if (onProgress) {
        xhr.upload.addEventListener('progress', (e) => {
          if (e.lengthComputable) onProgress(e.loaded / e.total);
        });
      }
      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try { resolve(JSON.parse(xhr.responseText)); }
          catch { resolve(null); }
        } else {
          if (xhr.status === 401) window.dispatchEvent(new Event('auth:session-expired'));
          try {
            const body = JSON.parse(xhr.responseText);
            reject(new Error(body.errors?.[0]?.message || `Upload failed: ${xhr.status}`));
          } catch { reject(new Error(`Upload failed: ${xhr.status}`)); }
        }
      });
      xhr.addEventListener('error', () => reject(new Error('Upload failed: network error')));
      xhr.addEventListener('abort', () => reject(new Error('Upload cancelled')));
      xhr.send(formData);
    });
  },
  getFolderFiles: async (folderId, page = 1, perPage = 20) => {
    return request(`/folders/${folderId}/files?page=${page}&per_page=${perPage}`);
  },

  // Folders
  getCourseFolders: async (courseId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/folders?page=${page}&per_page=${perPage}`);
  },
  createCourseFolder: async (courseId, folder) => {
    const { data } = await request(`/courses/${courseId}/folders`, {
      method: 'POST', body: JSON.stringify(folder),
    });
    return data;
  },
  getFolder: async (folderId) => {
    const { data } = await request(`/folders/${folderId}`);
    return data;
  },
  updateFolder: async (folderId, folder) => {
    const { data } = await request(`/folders/${folderId}`, {
      method: 'PUT', body: JSON.stringify(folder),
    });
    return data;
  },
  deleteFolder: async (folderId) => {
    const { data } = await request(`/folders/${folderId}`, { method: 'DELETE' });
    return data;
  },
  getSubfolders: async (folderId, page = 1, perPage = 50) => {
    return request(`/folders/${folderId}/folders?page=${page}&per_page=${perPage}`);
  },

  // SIS Imports
  createSISImport: async (accountId, importType, file) => {
    const formData = new FormData();
    formData.append('import_type', importType);
    formData.append('attachment', file);
    return uploadFile(`/accounts/${accountId}/sis_imports`, formData);
  },
  getSISImports: async (accountId = 1, page = 1, perPage = 20) => {
    return request(`/accounts/${accountId}/sis_imports?page=${page}&per_page=${perPage}`);
  },
  getSISImport: async (accountId, importId) => {
    const { data } = await request(`/accounts/${accountId}/sis_imports/${importId}`);
    return data;
  },
  getSISImportErrors: async (accountId, importId) => {
    const { data } = await request(`/accounts/${accountId}/sis_imports/${importId}/errors`);
    return data;
  },
  getSISExportUrl: (accountId, type) => `${API_URL}/accounts/${accountId}/sis_exports/${type}.csv`,

  // Quizzes
  getQuizzes: async (courseId, page = 1, perPage = 10) => {
    return request(`/courses/${courseId}/quizzes?page=${page}&per_page=${perPage}`);
  },
  getQuiz: async (courseId, quizId) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}`);
    return data;
  },
  createQuiz: async (courseId, quiz) => {
    const { data } = await request(`/courses/${courseId}/quizzes`, {
      method: 'POST', body: JSON.stringify({ quiz }),
    });
    return data;
  },
  updateQuiz: async (courseId, quizId, quiz) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}`, {
      method: 'PUT', body: JSON.stringify({ quiz }),
    });
    return data;
  },
  deleteQuiz: async (courseId, quizId) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}`, { method: 'DELETE' });
    return data;
  },

  // Quiz Questions
  getQuizQuestions: async (courseId, quizId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/quizzes/${quizId}/questions?page=${page}&per_page=${perPage}`);
  },
  getQuizQuestion: async (courseId, quizId, questionId) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/questions/${questionId}`);
    return data;
  },
  createQuizQuestion: async (courseId, quizId, question) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/questions`, {
      method: 'POST', body: JSON.stringify({ question }),
    });
    return data;
  },
  updateQuizQuestion: async (courseId, quizId, questionId, question) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/questions/${questionId}`, {
      method: 'PUT', body: JSON.stringify({ question }),
    });
    return data;
  },
  deleteQuizQuestion: async (courseId, quizId, questionId) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/questions/${questionId}`, { method: 'DELETE' });
    return data;
  },

  // Quiz Question Groups
  listQuizQuestionGroups: async (courseId, quizId) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/groups`);
    return data || [];
  },
  createQuizQuestionGroup: async (courseId, quizId, group) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/groups`, {
      method: 'POST', body: JSON.stringify(group),
    });
    return data;
  },
  updateQuizQuestionGroup: async (courseId, quizId, groupId, group) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/groups/${groupId}`, {
      method: 'PUT', body: JSON.stringify(group),
    });
    return data;
  },
  deleteQuizQuestionGroup: async (courseId, quizId, groupId) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/groups/${groupId}`, { method: 'DELETE' });
    return data;
  },

  // Quiz Submissions (backend wraps in {"quiz_submissions": [...]})
  startQuizSubmission: async (courseId, quizId) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/submissions`, { method: 'POST' });
    return data?.quiz_submissions?.[0] || data;
  },
  getQuizSubmission: async (courseId, quizId, submissionId) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/submissions/${submissionId}`);
    return data?.quiz_submissions?.[0] || data;
  },
  answerQuizQuestion: async (courseId, quizId, submissionId, questionId, answer) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/submissions/${submissionId}/questions/${questionId}`, {
      method: 'PUT', body: JSON.stringify({ answer }),
    });
    return data;
  },
  completeQuizSubmission: async (courseId, quizId, submissionId) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/submissions/${submissionId}/complete`, { method: 'POST' });
    return data?.quiz_submissions?.[0] || data;
  },
  getQuizSubmissions: async (courseId, quizId, page = 1, perPage = 50) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/submissions?page=${page}&per_page=${perPage}`);
    return { data: data?.quiz_submissions || data || [] };
  },
  getQuizSubmissionAnswers: async (courseId, quizId, submissionId) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/submissions/${submissionId}/answers`);
    return data?.quiz_submission_answers || data || [];
  },

  // Quiz Statistics (instructor only)
  getQuizStatistics: async (courseId, quizId) => {
    const { data } = await request(`/courses/${courseId}/quizzes/${quizId}/statistics`);
    return data;
  },

  // Rubrics
  getAssignmentRubric: async (courseId, assignmentId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/rubric`);
    return data;
  },
  getCourseRubrics: async (courseId, page = 1, perPage = 20) => {
    return request(`/courses/${courseId}/rubrics?page=${page}&per_page=${perPage}`);
  },
  getCourseRubric: async (courseId, rubricId) => {
    const { data } = await request(`/courses/${courseId}/rubrics/${rubricId}`);
    return data;
  },
  createCourseRubric: async (courseId, rubric) => {
    const { data } = await request(`/courses/${courseId}/rubrics`, {
      method: 'POST', body: JSON.stringify({ rubric }),
    });
    return data;
  },
  updateCourseRubric: async (courseId, rubricId, rubric) => {
    const { data } = await request(`/courses/${courseId}/rubrics/${rubricId}`, {
      method: 'PUT', body: JSON.stringify({ rubric }),
    });
    return data;
  },
  deleteCourseRubric: async (courseId, rubricId) => {
    const { data } = await request(`/courses/${courseId}/rubrics/${rubricId}`, { method: 'DELETE' });
    return data;
  },
  createRubricAssociation: async (courseId, rubricId, association) => {
    const { data } = await request(`/courses/${courseId}/rubrics/${rubricId}/associations`, {
      method: 'POST', body: JSON.stringify({ rubric_association: association }),
    });
    return data;
  },

  // Rubric Assessments
  getRubricAssessments: async (courseId, associationId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/rubric_associations/${associationId}/rubric_assessments?page=${page}&per_page=${perPage}`);
  },
  getRubricAssessment: async (courseId, associationId, assessmentId) => {
    const { data } = await request(`/courses/${courseId}/rubric_associations/${associationId}/rubric_assessments/${assessmentId}`);
    return data;
  },
  createRubricAssessment: async (courseId, associationId, assessment) => {
    const { data } = await request(`/courses/${courseId}/rubric_associations/${associationId}/rubric_assessments`, {
      method: 'POST', body: JSON.stringify({ rubric_assessment: assessment }),
    });
    return data;
  },
  updateRubricAssessment: async (courseId, associationId, assessmentId, assessment) => {
    const { data } = await request(`/courses/${courseId}/rubric_associations/${associationId}/rubric_assessments/${assessmentId}`, {
      method: 'PUT', body: JSON.stringify({ rubric_assessment: assessment }),
    });
    return data;
  },

  // Grading Period Groups
  getGradingPeriodGroups: async (accountId = 1, page = 1, perPage = 20) => {
    return request(`/accounts/${accountId}/grading_period_groups?page=${page}&per_page=${perPage}`);
  },
  getGradingPeriodGroup: async (accountId, groupId) => {
    const { data } = await request(`/accounts/${accountId}/grading_period_groups/${groupId}`);
    return data;
  },
  createGradingPeriodGroup: async (accountId, group) => {
    const { data } = await request(`/accounts/${accountId}/grading_period_groups`, {
      method: 'POST', body: JSON.stringify({ grading_period_group: group }),
    });
    return data;
  },
  updateGradingPeriodGroup: async (accountId, groupId, group) => {
    const { data } = await request(`/accounts/${accountId}/grading_period_groups/${groupId}`, {
      method: 'PUT', body: JSON.stringify({ grading_period_group: group }),
    });
    return data;
  },
  deleteGradingPeriodGroup: async (accountId, groupId) => {
    const { data } = await request(`/accounts/${accountId}/grading_period_groups/${groupId}`, { method: 'DELETE' });
    return data;
  },

  // Grading Periods
  getGradingPeriods: async (accountId, groupId) => {
    const { data } = await request(`/accounts/${accountId}/grading_period_groups/${groupId}/grading_periods`);
    return data;
  },
  getGradingPeriod: async (accountId, groupId, periodId) => {
    const { data } = await request(`/accounts/${accountId}/grading_period_groups/${groupId}/grading_periods/${periodId}`);
    return data;
  },
  createGradingPeriod: async (accountId, groupId, period) => {
    const { data } = await request(`/accounts/${accountId}/grading_period_groups/${groupId}/grading_periods`, {
      method: 'POST', body: JSON.stringify({ grading_period: period }),
    });
    return data;
  },
  updateGradingPeriod: async (accountId, groupId, periodId, period) => {
    const { data } = await request(`/accounts/${accountId}/grading_period_groups/${groupId}/grading_periods/${periodId}`, {
      method: 'PUT', body: JSON.stringify({ grading_period: period }),
    });
    return data;
  },
  deleteGradingPeriod: async (accountId, groupId, periodId) => {
    const { data } = await request(`/accounts/${accountId}/grading_period_groups/${groupId}/grading_periods/${periodId}`, { method: 'DELETE' });
    return data;
  },

  // Assignment Overrides
  getAssignmentOverrides: async (courseId, assignmentId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/overrides`);
    return data;
  },
  getAssignmentOverride: async (courseId, assignmentId, overrideId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/overrides/${overrideId}`);
    return data;
  },
  createAssignmentOverride: async (courseId, assignmentId, override) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/overrides`, {
      method: 'POST', body: JSON.stringify({ assignment_override: override }),
    });
    return data;
  },
  updateAssignmentOverride: async (courseId, assignmentId, overrideId, override) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/overrides/${overrideId}`, {
      method: 'PUT', body: JSON.stringify({ assignment_override: override }),
    });
    return data;
  },
  deleteAssignmentOverride: async (courseId, assignmentId, overrideId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/overrides/${overrideId}`, { method: 'DELETE' });
    return data;
  },

  // Late Policy
  getLatePolicy: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/late_policy`);
    return data;
  },
  createLatePolicy: async (courseId, policy) => {
    const { data } = await request(`/courses/${courseId}/late_policy`, {
      method: 'POST', body: JSON.stringify({ late_policy: policy }),
    });
    return data;
  },
  updateLatePolicy: async (courseId, policy) => {
    const { data } = await request(`/courses/${courseId}/late_policy`, {
      method: 'PUT', body: JSON.stringify({ late_policy: policy }),
    });
    return data;
  },
  deleteLatePolicy: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/late_policy`, { method: 'DELETE' });
    return data;
  },

  // Calendar Events
  getCalendarEvents: async (page = 1, perPage = 20) => {
    return request(`/calendar_events?page=${page}&per_page=${perPage}`);
  },
  getCourseCalendarEvents: async (courseId, page = 1, perPage = 20) => {
    return request(`/courses/${courseId}/calendar_events?page=${page}&per_page=${perPage}`);
  },
  getCalendarEvent: async (id) => {
    const { data } = await request(`/calendar_events/${id}`);
    return data;
  },
  createCalendarEvent: async (event) => {
    const { data } = await request('/calendar_events', {
      method: 'POST', body: JSON.stringify({ calendar_event: event }),
    });
    return data;
  },
  updateCalendarEvent: async (id, event) => {
    const { data } = await request(`/calendar_events/${id}`, {
      method: 'PUT', body: JSON.stringify({ calendar_event: event }),
    });
    return data;
  },
  deleteCalendarEvent: async (id) => {
    const { data } = await request(`/calendar_events/${id}`, { method: 'DELETE' });
    return data;
  },
  getCalendarICalUrl: () => `${API_URL}/calendar_events.ics`,

  // Conversations
  getConversations: async (page = 1, perPage = 20) => {
    return request(`/conversations?page=${page}&per_page=${perPage}`);
  },
  getConversation: async (id) => {
    const { data } = await request(`/conversations/${id}`);
    return data;
  },
  createConversation: async (conversation) => {
    const { data } = await request('/conversations', {
      method: 'POST', body: JSON.stringify({ conversation }),
    });
    return data;
  },
  updateConversation: async (id, conversation) => {
    const { data } = await request(`/conversations/${id}`, {
      method: 'PUT', body: JSON.stringify({ conversation }),
    });
    return data;
  },
  getConversationMessages: async (conversationId, page = 1, perPage = 50) => {
    return request(`/conversations/${conversationId}/messages?page=${page}&per_page=${perPage}`);
  },
  createConversationMessage: async (conversationId, message) => {
    const { data } = await request(`/conversations/${conversationId}/messages`, {
      method: 'POST', body: JSON.stringify({ message }),
    });
    return data;
  },
  markConversationAsRead: async (conversationId) => {
    const { data } = await request(`/conversations/${conversationId}/mark_as_read`, {
      method: 'PUT', body: JSON.stringify({}),
    });
    return data;
  },

  // Notifications
  getNotifications: async (page = 1, perPage = 20, unread = false) => {
    const params = new URLSearchParams({ page, per_page: perPage });
    if (unread) params.append('unread', 'true');
    return request(`/notifications?${params.toString()}`);
  },
  markNotificationAsRead: async (id) => {
    const { data } = await request(`/notifications/${id}/mark_as_read`, {
      method: 'PUT', body: JSON.stringify({}),
    });
    return data;
  },
  markAllNotificationsAsRead: async () => {
    const { data } = await request('/notifications/mark_all_as_read', {
      method: 'PUT', body: JSON.stringify({}),
    });
    return data;
  },
  getNotificationPreferences: async () => {
    const { data } = await request('/users/self/notification_preferences');
    return data;
  },
  updateNotificationPreferences: async (prefs) => {
    const { data } = await request('/users/self/notification_preferences', {
      method: 'PUT', body: JSON.stringify({ notification_preference: prefs }),
    });
    return data;
  },

  // Content Migrations
  getContentMigrations: async (courseId, page = 1, perPage = 20) => {
    return request(`/courses/${courseId}/content_migrations?page=${page}&per_page=${perPage}`);
  },
  getContentMigration: async (courseId, migrationId) => {
    const { data } = await request(`/courses/${courseId}/content_migrations/${migrationId}`);
    return data;
  },
  createContentMigration: async (courseId, migration) => {
    const { data } = await request(`/courses/${courseId}/content_migrations`, {
      method: 'POST', body: JSON.stringify({ content_migration: migration }),
    });
    return data;
  },
  updateContentMigration: async (courseId, migrationId, migration) => {
    const { data } = await request(`/courses/${courseId}/content_migrations/${migrationId}`, {
      method: 'PUT', body: JSON.stringify({ content_migration: migration }),
    });
    return data;
  },
  uploadContentMigration: async (courseId, migrationType, file) => {
    const formData = new FormData();
    formData.append('migration_type', migrationType);
    formData.append('attachment', file);
    return uploadFile(`/courses/${courseId}/content_migrations`, formData);
  },

  // Learning Outcome Groups
  getCourseOutcomeGroups: async (courseId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/outcome_groups?page=${page}&per_page=${perPage}`);
  },
  getCourseOutcomeGroup: async (courseId, groupId) => {
    const { data } = await request(`/courses/${courseId}/outcome_groups/${groupId}`);
    return data;
  },
  createCourseOutcomeGroup: async (courseId, group) => {
    const { data } = await request(`/courses/${courseId}/outcome_groups`, {
      method: 'POST', body: JSON.stringify(group),
    });
    return data;
  },
  updateCourseOutcomeGroup: async (courseId, groupId, group) => {
    const { data } = await request(`/courses/${courseId}/outcome_groups/${groupId}`, {
      method: 'PUT', body: JSON.stringify(group),
    });
    return data;
  },
  deleteCourseOutcomeGroup: async (courseId, groupId) => {
    const { data } = await request(`/courses/${courseId}/outcome_groups/${groupId}`, { method: 'DELETE' });
    return data;
  },

  // Learning Outcomes
  getOutcomeGroupOutcomes: async (courseId, groupId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/outcome_groups/${groupId}/outcomes?page=${page}&per_page=${perPage}`);
  },
  createOutcome: async (courseId, groupId, outcome) => {
    const { data } = await request(`/courses/${courseId}/outcome_groups/${groupId}/outcomes`, {
      method: 'POST', body: JSON.stringify(outcome),
    });
    return data;
  },
  getOutcome: async (courseId, outcomeId) => {
    const { data } = await request(`/courses/${courseId}/outcomes/${outcomeId}`);
    return data;
  },
  updateOutcome: async (courseId, outcomeId, outcome) => {
    const { data } = await request(`/courses/${courseId}/outcomes/${outcomeId}`, {
      method: 'PUT', body: JSON.stringify(outcome),
    });
    return data;
  },
  deleteOutcome: async (courseId, outcomeId) => {
    const { data } = await request(`/courses/${courseId}/outcomes/${outcomeId}`, { method: 'DELETE' });
    return data;
  },

  // Learning Outcome Alignments
  getOutcomeAlignments: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/outcome_alignments`);
    return data || [];
  },
  createOutcomeAlignment: async (courseId, alignment) => {
    const { data } = await request(`/courses/${courseId}/outcome_alignments`, {
      method: 'POST', body: JSON.stringify(alignment),
    });
    return data;
  },
  deleteOutcomeAlignment: async (courseId, alignmentId) => {
    const { data } = await request(`/courses/${courseId}/outcome_alignments/${alignmentId}`, { method: 'DELETE' });
    return data;
  },
  getCourseOutcomes: async (courseId) => {
    const groupsResult = await request(`/courses/${courseId}/outcome_groups?page=1&per_page=100`);
    const groups = groupsResult.data || [];
    const outcomes = [];
    for (const group of groups) {
      try {
        const outcomeResult = await request(`/courses/${courseId}/outcome_groups/${group.id}/outcomes?page=1&per_page=100`);
        const groupOutcomes = outcomeResult.data || [];
        for (const o of groupOutcomes) {
          outcomes.push({ ...o, group_title: group.title });
        }
      } catch {
        // skip groups that fail
      }
    }
    return outcomes;
  },

  // Learning Outcome Results
  getOutcomeResults: async (courseId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/outcome_results?page=${page}&per_page=${perPage}`);
  },
  createOutcomeResult: async (courseId, result) => {
    const { data } = await request(`/courses/${courseId}/outcome_results`, {
      method: 'POST', body: JSON.stringify(result),
    });
    return data;
  },
  getOutcomeRollups: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/outcome_rollups`);
    return data;
  },

  // SpeedGrader
  getSpeedGraderData: async (courseId, assignmentId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/speedgrader`);
    return data;
  },
  getSpeedGraderStudentSubmission: async (courseId, assignmentId, userId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/speedgrader/submissions/${userId}`);
    return data;
  },

  // Groups
  getGroupCategories: async (courseId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/group_categories?page=${page}&per_page=${perPage}`);
  },
  createGroupCategory: async (courseId, category) => {
    const { data } = await request(`/courses/${courseId}/group_categories`, {
      method: 'POST', body: JSON.stringify({ group_category: category }),
    });
    return data;
  },
  getGroupCategory: async (categoryId) => {
    const { data } = await request(`/group_categories/${categoryId}`);
    return data;
  },
  updateGroupCategory: async (categoryId, category) => {
    const { data } = await request(`/group_categories/${categoryId}`, {
      method: 'PUT', body: JSON.stringify({ group_category: category }),
    });
    return data;
  },
  deleteGroupCategory: async (categoryId) => {
    const { data } = await request(`/group_categories/${categoryId}`, { method: 'DELETE' });
    return data;
  },
  getGroupsByCategory: async (categoryId, page = 1, perPage = 50) => {
    return request(`/group_categories/${categoryId}/groups?page=${page}&per_page=${perPage}`);
  },
  createGroup: async (categoryId, group) => {
    const { data } = await request(`/group_categories/${categoryId}/groups`, {
      method: 'POST', body: JSON.stringify({ group }),
    });
    return data;
  },
  getGroup: async (groupId) => {
    const { data } = await request(`/groups/${groupId}`);
    return data;
  },
  updateGroup: async (groupId, group) => {
    const { data } = await request(`/groups/${groupId}`, {
      method: 'PUT', body: JSON.stringify({ group }),
    });
    return data;
  },
  deleteGroup: async (groupId) => {
    const { data } = await request(`/groups/${groupId}`, { method: 'DELETE' });
    return data;
  },
  getGroupMemberships: async (groupId, page = 1, perPage = 50) => {
    return request(`/groups/${groupId}/memberships?page=${page}&per_page=${perPage}`);
  },
  createGroupMembership: async (groupId, membership) => {
    const { data } = await request(`/groups/${groupId}/memberships`, {
      method: 'POST', body: JSON.stringify({ membership }),
    });
    return data;
  },
  updateGroupMembership: async (groupId, membershipId, membership) => {
    const { data } = await request(`/groups/${groupId}/memberships/${membershipId}`, {
      method: 'PUT', body: JSON.stringify({ membership }),
    });
    return data;
  },
  deleteGroupMembership: async (groupId, membershipId) => {
    const { data } = await request(`/groups/${groupId}/memberships/${membershipId}`, { method: 'DELETE' });
    return data;
  },
  getUserGroups: async (page = 1, perPage = 50) => {
    return request(`/users/self/groups?page=${page}&per_page=${perPage}`);
  },

  // Blueprint Courses
  getBlueprintTemplates: async (courseId, page = 1, perPage = 10) => {
    return request(`/courses/${courseId}/blueprint_templates?page=${page}&per_page=${perPage}`);
  },
  createBlueprintTemplate: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/blueprint_templates`, { method: 'POST' });
    return data;
  },
  getDefaultBlueprintTemplate: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/blueprint_templates/default`);
    return data;
  },
  updateDefaultBlueprintTemplate: async (courseId, template) => {
    const { data } = await request(`/courses/${courseId}/blueprint_templates/default`, {
      method: 'PUT', body: JSON.stringify({ blueprint_template: template }),
    });
    return data;
  },
  getBlueprintAssociatedCourses: async (courseId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/blueprint_templates/default/associated_courses?page=${page}&per_page=${perPage}`);
  },
  updateBlueprintAssociations: async (courseId, courseIdsToAdd, courseIdsToRemove) => {
    const { data } = await request(`/courses/${courseId}/blueprint_templates/default/associated_courses`, {
      method: 'PUT', body: JSON.stringify({ course_ids_to_add: courseIdsToAdd, course_ids_to_remove: courseIdsToRemove }),
    });
    return data;
  },
  getBlueprintMigrations: async (courseId, page = 1, perPage = 20) => {
    return request(`/courses/${courseId}/blueprint_templates/default/migrations?page=${page}&per_page=${perPage}`);
  },
  createBlueprintMigration: async (courseId, comment) => {
    const { data } = await request(`/courses/${courseId}/blueprint_templates/default/migrations`, {
      method: 'POST', body: JSON.stringify({ comment }),
    });
    return data;
  },
  getBlueprintMigration: async (courseId, migrationId) => {
    const { data } = await request(`/courses/${courseId}/blueprint_templates/default/migrations/${migrationId}`);
    return data;
  },
  getBlueprintUnsyncedChanges: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/blueprint_templates/default/unsynced_changes`);
    return data;
  },
  getBlueprintSubscriptions: async (courseId, page = 1, perPage = 20) => {
    return request(`/courses/${courseId}/blueprint_subscriptions?page=${page}&per_page=${perPage}`);
  },

  // Course Pacing
  getCoursePaces: async (courseId, page = 1, perPage = 20) => {
    return request(`/courses/${courseId}/course_pacing?page=${page}&per_page=${perPage}`);
  },
  createCoursePace: async (courseId, pace) => {
    const { data } = await request(`/courses/${courseId}/course_pacing`, {
      method: 'POST', body: JSON.stringify({ course_pace: pace }),
    });
    return data;
  },
  getCoursePace: async (courseId, paceId) => {
    const { data } = await request(`/courses/${courseId}/course_pacing/${paceId}`);
    return data;
  },
  updateCoursePace: async (courseId, paceId, pace) => {
    const { data } = await request(`/courses/${courseId}/course_pacing/${paceId}`, {
      method: 'PUT', body: JSON.stringify({ course_pace: pace }),
    });
    return data;
  },
  deleteCoursePace: async (courseId, paceId) => {
    const { data } = await request(`/courses/${courseId}/course_pacing/${paceId}`, { method: 'DELETE' });
    return data;
  },
  publishCoursePace: async (courseId, paceId) => {
    const { data } = await request(`/courses/${courseId}/course_pacing/${paceId}/publish`, { method: 'POST' });
    return data;
  },
  getCoursePaceModuleItems: async (courseId, paceId) => {
    const { data } = await request(`/courses/${courseId}/course_pacing/${paceId}/module_items`);
    return data;
  },
  updateCoursePaceModuleItems: async (courseId, paceId, items) => {
    const { data } = await request(`/courses/${courseId}/course_pacing/${paceId}/module_items`, {
      method: 'PUT', body: JSON.stringify({ module_items: items }),
    });
    return data;
  },
  getCoursePaceTimeline: async (courseId, paceId) => {
    return request(`/courses/${courseId}/course_pacing/${paceId}/module_items`);
  },

  // Collaborations
  getCollaborations: async (courseId, page = 1, perPage = 20) => {
    return request(`/courses/${courseId}/collaborations?page=${page}&per_page=${perPage}`);
  },
  createCollaboration: async (courseId, collaboration) => {
    const { data } = await request(`/courses/${courseId}/collaborations`, {
      method: 'POST', body: JSON.stringify({ collaboration }),
    });
    return data;
  },
  getCollaboration: async (id) => {
    const { data } = await request(`/collaborations/${id}`);
    return data;
  },
  updateCollaboration: async (id, collaboration) => {
    const { data } = await request(`/collaborations/${id}`, {
      method: 'PUT', body: JSON.stringify({ collaboration }),
    });
    return data;
  },
  deleteCollaboration: async (id) => {
    const { data } = await request(`/collaborations/${id}`, { method: 'DELETE' });
    return data;
  },

  // Conferences
  getConferences: async (courseId, page = 1, perPage = 20) => {
    return request(`/courses/${courseId}/conferences?page=${page}&per_page=${perPage}`);
  },
  createConference: async (courseId, conference) => {
    const { data } = await request(`/courses/${courseId}/conferences`, {
      method: 'POST', body: JSON.stringify({ conference }),
    });
    return data;
  },
  getConference: async (id) => {
    const { data } = await request(`/conferences/${id}`);
    return data;
  },
  updateConference: async (id, conference) => {
    const { data } = await request(`/conferences/${id}`, {
      method: 'PUT', body: JSON.stringify({ conference }),
    });
    return data;
  },
  deleteConference: async (id) => {
    const { data } = await request(`/conferences/${id}`, { method: 'DELETE' });
    return data;
  },
  joinConference: async (id) => {
    const { data } = await request(`/conferences/${id}/join`, { method: 'POST' });
    return data;
  },
  endConference: async (id) => {
    const { data } = await request(`/conferences/${id}/end`, { method: 'POST' });
    return data;
  },
  getConferenceRecordings: async (id) => {
    const { data } = await request(`/conferences/${id}/recordings`);
    return data;
  },
  getConferenceParticipants: async (id) => {
    const { data } = await request(`/conferences/${id}/participants`);
    return data;
  },

  // Analytics
  getCourseAnalyticsActivity: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/analytics/activity`);
    return data;
  },
  getCourseAnalyticsAssignments: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/analytics/assignments`);
    return data;
  },
  getCourseAnalyticsStudentSummaries: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/analytics/student_summaries`);
    return data;
  },
  getStudentAnalyticsActivity: async (courseId, userId) => {
    const { data } = await request(`/courses/${courseId}/analytics/users/${userId}/activity`);
    return data;
  },
  getStudentAnalyticsAssignments: async (courseId, userId) => {
    const { data } = await request(`/courses/${courseId}/analytics/users/${userId}/assignments`);
    return data;
  },
  getDepartmentAnalyticsActivity: async (accountId = 1) => {
    const { data } = await request(`/accounts/${accountId}/analytics/current/activity`);
    return data;
  },
  getDepartmentAnalyticsGrades: async (accountId = 1) => {
    const { data } = await request(`/accounts/${accountId}/analytics/current/grades`);
    return data;
  },
  getDepartmentAnalyticsStatistics: async (accountId = 1) => {
    const { data } = await request(`/accounts/${accountId}/analytics/current/statistics`);
    return data;
  },
  createPageView: async (pageView) => {
    const { data } = await request('/page_views', {
      method: 'POST', body: JSON.stringify({ page_view: pageView }),
    });
    return data;
  },
  getUserPageViews: async (page = 1, perPage = 50) => {
    return request(`/users/self/page_views?page=${page}&per_page=${perPage}`);
  },

  // Observer/Parent Role
  linkObservee: async (userId, observeeId) => {
    const { data } = await request(`/users/${userId}/observees`, {
      method: 'POST', body: JSON.stringify({ observee_id: observeeId }),
    });
    return data;
  },
  unlinkObservee: async (userId, observeeId) => {
    const { data } = await request(`/users/${userId}/observees/${observeeId}`, { method: 'DELETE' });
    return data;
  },
  getObservees: async (userId) => {
    const { data } = await request(`/users/${userId}/observees`);
    return data;
  },
  getObserveeCourses: async (userId, observeeId) => {
    const { data } = await request(`/users/${userId}/observees/${observeeId}/courses`);
    return data;
  },
  getChildOverview: (parentId, childId) =>
    request(`/users/${parentId}/observees/${childId}/overview`),

  // Parent/observer pairing codes
  generatePairingCode: () =>
    request('/users/self/pairing_codes', { method: 'POST' }),
  redeemPairingCode: (code) =>
    request('/users/self/pairing_codes/redeem', {
      method: 'POST',
      body: JSON.stringify({ code }),
    }),
  listPairingCodes: () =>
    request('/users/self/pairing_codes'),
  revokePairingCode: (id) =>
    request(`/users/self/pairing_codes/${id}`, { method: 'DELETE' }),

  // GraphQL
  graphql: async (query, variables = {}) => {
    const { data } = await request('/graphql', {
      method: 'POST', body: JSON.stringify({ query, variables }),
    });
    return data;
  },

  // Authentication Providers
  getAuthProviders: async (accountId = 1, page = 1, perPage = 20) => {
    return request(`/accounts/${accountId}/authentication_providers?page=${page}&per_page=${perPage}`);
  },
  getAuthProvider: async (accountId, providerId) => {
    const { data } = await request(`/accounts/${accountId}/authentication_providers/${providerId}`);
    return data;
  },
  createAuthProvider: async (accountId, provider) => {
    const { data } = await request(`/accounts/${accountId}/authentication_providers`, {
      method: 'POST', body: JSON.stringify({ authentication_provider: provider }),
    });
    return data;
  },
  updateAuthProvider: async (accountId, providerId, provider) => {
    const { data } = await request(`/accounts/${accountId}/authentication_providers/${providerId}`, {
      method: 'PUT', body: JSON.stringify({ authentication_provider: provider }),
    });
    return data;
  },
  deleteAuthProvider: async (accountId, providerId) => {
    const { data } = await request(`/accounts/${accountId}/authentication_providers/${providerId}`, { method: 'DELETE' });
    return data;
  },
  testAuthProviderConnection: async (accountId, providerId) => {
    const { data } = await request(`/accounts/${accountId}/authentication_providers/${providerId}/test`, { method: 'POST' });
    return data;
  },

  // Discussion V2 (enhanced)
  getDiscussionFullViewV2: async (courseId, topicId) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/view_v2`);
    return data;
  },
  markDiscussionEntryRead: async (courseId, topicId, entryId) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/entries/${entryId}/read`, { method: 'POST' });
    return data;
  },
  markDiscussionTopicRead: async (courseId, topicId) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/mark_all_read`, { method: 'POST' });
    return data;
  },
  getDiscussionUnreadCount: async (courseId, topicId) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/unread_count`);
    return data;
  },
  toggleDiscussionSubscription: async (courseId, topicId, subscribed) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/subscription`, {
      method: 'PUT', body: JSON.stringify({ subscribed }),
    });
    return data;
  },
  getDiscussionEntryVersions: async (courseId, topicId, entryId) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/entries/${entryId}/versions`);
    return data;
  },
  updateDiscussionEntryV2: async (courseId, topicId, entryId, message) => {
    const { data } = await request(`/courses/${courseId}/discussion_topics/${topicId}/entries/${entryId}/v2`, {
      method: 'PUT', body: JSON.stringify({ message }),
    });
    return data;
  },

  // Content Import (IMSCC/Common Cartridge)
  importContentPackage: async (courseId, file) => {
    const formData = new FormData();
    formData.append('file', file);
    return uploadFile(`/courses/${courseId}/content_imports`, formData);
  },

  // Batch Operations
  cloneCourse: async (sourceCourseId, name, accountId = 1, include = {}) => {
    const { data } = await request('/courses/clone', {
      method: 'POST', body: JSON.stringify({ source_course_id: sourceCourseId, name, account_id: accountId, include }),
    });
    return data;
  },
  bulkDateShift: async (courseId, oldStartDate, newStartDate, dayShift) => {
    const { data } = await request(`/courses/${courseId}/date_shift`, {
      method: 'POST', body: JSON.stringify({ old_start_date: oldStartDate, new_start_date: newStartDate, day_shift: dayShift }),
    });
    return data;
  },
  bulkSendMessage: async (courseId, enrollmentTypes, subject, body) => {
    const { data } = await request('/conversations/bulk', {
      method: 'POST', body: JSON.stringify({ course_id: courseId, enrollment_types: enrollmentTypes, subject, body }),
    });
    return data;
  },
  bulkEnrollUsers: async (courseId, enrollments) => {
    const { data } = await request(`/courses/${courseId}/enrollments/bulk`, {
      method: 'POST', body: JSON.stringify({ enrollments }),
    });
    return data;
  },
  bulkUpdateAssignmentDates: async (courseId, updates) => {
    const { data } = await request(`/courses/${courseId}/assignments/bulk_update_dates`, {
      method: 'POST', body: JSON.stringify({ updates }),
    });
    return data;
  },

  // Announcements
  getCourseAnnouncements: async (courseId, page = 1, perPage = 20) => {
    return request(`/courses/${courseId}/announcements?page=${page}&per_page=${perPage}`);
  },
  createCourseAnnouncement: async (courseId, announcement) => {
    const { data } = await request(`/courses/${courseId}/announcements`, {
      method: 'POST', body: JSON.stringify({ announcement }),
    });
    return data;
  },
  getAnnouncement: async (id) => {
    const { data } = await request(`/announcements/${id}`);
    return data;
  },
  updateAnnouncement: async (id, announcement) => {
    const { data } = await request(`/announcements/${id}`, {
      method: 'PUT', body: JSON.stringify({ announcement }),
    });
    return data;
  },
  deleteAnnouncement: async (id) => {
    const { data } = await request(`/announcements/${id}`, { method: 'DELETE' });
    return data;
  },
  markAnnouncementRead: async (id) => {
    const { data } = await request(`/announcements/${id}/read`, { method: 'POST' });
    return data;
  },
  acknowledgeAnnouncement: async (id) => {
    const { data } = await request(`/announcements/${id}/acknowledge`, { method: 'POST' });
    return data;
  },
  getAnnouncementReadReceipts: async (id, page = 1, perPage = 50) => {
    return request(`/announcements/${id}/read_receipts?page=${page}&per_page=${perPage}`);
  },
  getAccountAnnouncements: async (accountId = 1, page = 1, perPage = 20) => {
    return request(`/accounts/${accountId}/announcements?page=${page}&per_page=${perPage}`);
  },
  createAccountAnnouncement: async (accountId, announcement) => {
    const { data } = await request(`/accounts/${accountId}/announcements`, {
      method: 'POST', body: JSON.stringify({ announcement }),
    });
    return data;
  },

  // Enrollment Terms
  getEnrollmentTerms: async (accountId = 1, page = 1, perPage = 20) => {
    return request(`/accounts/${accountId}/terms?page=${page}&per_page=${perPage}`);
  },
  createEnrollmentTerm: async (accountId, term) => {
    const { data } = await request(`/accounts/${accountId}/terms`, {
      method: 'POST', body: JSON.stringify({ enrollment_term: term }),
    });
    return data;
  },
  getEnrollmentTerm: async (accountId, termId) => {
    const { data } = await request(`/accounts/${accountId}/terms/${termId}`);
    return data;
  },
  updateEnrollmentTerm: async (accountId, termId, term) => {
    const { data } = await request(`/accounts/${accountId}/terms/${termId}`, {
      method: 'PUT', body: JSON.stringify({ enrollment_term: term }),
    });
    return data;
  },
  deleteEnrollmentTerm: async (accountId, termId) => {
    const { data } = await request(`/accounts/${accountId}/terms/${termId}`, { method: 'DELETE' });
    return data;
  },
  getCurrentEnrollmentTerm: async (accountId = 1) => {
    const { data } = await request(`/accounts/${accountId}/terms/current`);
    return data;
  },

  // Syllabus
  getCourseSyllabus: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/syllabus`);
    return data;
  },

  // Notification Delivery
  getNotificationDeliveries: async (page = 1, perPage = 20, status = '') => {
    const params = new URLSearchParams({ page, per_page: perPage });
    if (status) params.append('status', status);
    return request(`/users/self/notification_deliveries?${params.toString()}`);
  },
  getNotificationDeliveryStats: async () => {
    const { data } = await request('/admin/notification_stats');
    return data;
  },
  retryFailedDeliveries: async () => {
    const { data } = await request('/admin/notification_deliveries/retry', { method: 'POST' });
    return data;
  },

  // Communication Channels
  getCommunicationChannels: async () => {
    const { data } = await request('/users/self/communication_channels');
    return data;
  },
  createCommunicationChannel: async (channelType, address) => {
    const { data } = await request('/users/self/communication_channels', {
      method: 'POST', body: JSON.stringify({ communication_channel: { channel_type: channelType, address } }),
    });
    return data;
  },
  deleteCommunicationChannel: async (id) => {
    const { data } = await request(`/users/self/communication_channels/${id}`, { method: 'DELETE' });
    return data;
  },

  // Audit Logs
  getCourseAuditLog: async (courseId, page = 1, perPage = 20, filters = {}) => {
    const params = new URLSearchParams({ page, per_page: perPage });
    if (filters.event_type) params.append('event_type', filters.event_type);
    if (filters.user_id) params.append('user_id', filters.user_id);
    if (filters.date_from) params.append('date_from', filters.date_from);
    if (filters.date_to) params.append('date_to', filters.date_to);
    return request(`/courses/${courseId}/audit_log?${params.toString()}`);
  },
  getCourseGradeChangeLog: async (courseId, page = 1, perPage = 20, filters = {}) => {
    const params = new URLSearchParams({ page, per_page: perPage });
    if (filters.student_id) params.append('student_id', filters.student_id);
    if (filters.grader_id) params.append('grader_id', filters.grader_id);
    if (filters.assignment_id) params.append('assignment_id', filters.assignment_id);
    if (filters.date_from) params.append('date_from', filters.date_from);
    if (filters.date_to) params.append('date_to', filters.date_to);
    return request(`/courses/${courseId}/grade_change_log?${params.toString()}`);
  },
  getAccountAuditLog: async (accountId = 1, page = 1, perPage = 20, filters = {}) => {
    const params = new URLSearchParams({ page, per_page: perPage });
    if (filters.event_type) params.append('event_type', filters.event_type);
    if (filters.user_id) params.append('user_id', filters.user_id);
    if (filters.date_from) params.append('date_from', filters.date_from);
    if (filters.date_to) params.append('date_to', filters.date_to);
    return request(`/accounts/${accountId}/audit_log?${params.toString()}`);
  },
  getAuditLogSummary: async (dateFrom, dateTo) => {
    const params = new URLSearchParams();
    if (dateFrom) params.append('date_from', dateFrom);
    if (dateTo) params.append('date_to', dateTo);
    const { data } = await request(`/admin/audit_log/summary?${params.toString()}`);
    return data;
  },
  exportCourseAuditLogCSV: (courseId) => `${API_URL}/courses/${courseId}/audit_log.csv`,
  exportCourseGradeChangeLogCSV: (courseId) => `${API_URL}/courses/${courseId}/grade_change_log.csv`,

  // Custom Roles
  getCustomRoles: async (accountId = 1, page = 1, perPage = 20) => {
    return request(`/accounts/${accountId}/roles?page=${page}&per_page=${perPage}`);
  },
  createCustomRole: async (accountId, role) => {
    const { data } = await request(`/accounts/${accountId}/roles`, {
      method: 'POST', body: JSON.stringify({ role }),
    });
    return data;
  },
  getCustomRole: async (accountId, roleId) => {
    const { data } = await request(`/accounts/${accountId}/roles/${roleId}`);
    return data;
  },
  updateCustomRole: async (accountId, roleId, role) => {
    const { data } = await request(`/accounts/${accountId}/roles/${roleId}`, {
      method: 'PUT', body: JSON.stringify({ role }),
    });
    return data;
  },
  deleteCustomRole: async (accountId, roleId) => {
    const { data } = await request(`/accounts/${accountId}/roles/${roleId}`, { method: 'DELETE' });
    return data;
  },
  cloneCustomRole: async (accountId, roleId, name) => {
    const { data } = await request(`/accounts/${accountId}/roles/${roleId}/clone`, {
      method: 'POST', body: JSON.stringify({ name }),
    });
    return data;
  },
  getPermissionPresets: async (accountId = 1) => {
    const { data } = await request(`/accounts/${accountId}/roles/presets`);
    return data;
  },
  getRoleOverrides: async (accountId, roleId) => {
    const { data } = await request(`/accounts/${accountId}/roles/${roleId}/overrides`);
    return data;
  },
  bulkSetRoleOverrides: async (accountId, roleId, overrides) => {
    const { data } = await request(`/accounts/${accountId}/roles/${roleId}/overrides`, {
      method: 'PUT', body: JSON.stringify({ overrides }),
    });
    return data;
  },
  getCoursePermissions: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/permissions`);
    return data;
  },

  // OneRoster
  getOneRosterConnections: async (accountId = 1, page = 1, perPage = 20) => {
    return request(`/accounts/${accountId}/oneroster_connections?page=${page}&per_page=${perPage}`);
  },
  createOneRosterConnection: async (accountId, connection) => {
    const { data } = await request(`/accounts/${accountId}/oneroster_connections`, {
      method: 'POST', body: JSON.stringify({ connection }),
    });
    return data;
  },
  getOneRosterConnection: async (accountId, connectionId) => {
    const { data } = await request(`/accounts/${accountId}/oneroster_connections/${connectionId}`);
    return data;
  },
  updateOneRosterConnection: async (accountId, connectionId, connection) => {
    const { data } = await request(`/accounts/${accountId}/oneroster_connections/${connectionId}`, {
      method: 'PUT', body: JSON.stringify({ connection }),
    });
    return data;
  },
  deleteOneRosterConnection: async (accountId, connectionId) => {
    const { data } = await request(`/accounts/${accountId}/oneroster_connections/${connectionId}`, { method: 'DELETE' });
    return data;
  },
  testOneRosterConnection: async (accountId, connectionId) => {
    const { data } = await request(`/accounts/${accountId}/oneroster_connections/${connectionId}/test`, { method: 'POST' });
    return data;
  },
  syncOneRosterFull: async (accountId, connectionId) => {
    const { data } = await request(`/accounts/${accountId}/oneroster_connections/${connectionId}/sync`, { method: 'POST' });
    return data;
  },
  syncOneRosterIncremental: async (accountId, connectionId) => {
    const { data } = await request(`/accounts/${accountId}/oneroster_connections/${connectionId}/sync_incremental`, { method: 'POST' });
    return data;
  },
  getOneRosterSyncLogs: async (accountId, connectionId, page = 1, perPage = 20) => {
    return request(`/accounts/${accountId}/oneroster_connections/${connectionId}/sync_logs?page=${page}&per_page=${perPage}`);
  },

  // Document Annotations
  getAnnotations: async (courseId, assignmentId, userId, page = 1, perPage = 100) => {
    return request(`/courses/${courseId}/assignments/${assignmentId}/submissions/${userId}/annotations?page=${page}&per_page=${perPage}`);
  },
  createAnnotation: async (courseId, assignmentId, userId, annotation) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/submissions/${userId}/annotations`, {
      method: 'POST', body: JSON.stringify({ annotation }),
    });
    return data;
  },
  getAnnotation: async (id) => {
    const { data } = await request(`/annotations/${id}`);
    return data;
  },
  updateAnnotation: async (id, annotation) => {
    const { data } = await request(`/annotations/${id}`, {
      method: 'PUT', body: JSON.stringify({ annotation }),
    });
    return data;
  },
  deleteAnnotation: async (id, courseId) => {
    const { data } = await request(`/annotations/${id}?course_id=${courseId}`, { method: 'DELETE' });
    return data;
  },
  resolveAnnotation: async (id) => {
    const { data } = await request(`/annotations/${id}/resolve`, { method: 'POST' });
    return data;
  },
  unresolveAnnotation: async (id) => {
    const { data } = await request(`/annotations/${id}/resolve`, { method: 'DELETE' });
    return data;
  },
  replyToAnnotation: async (id, content, courseId) => {
    const { data } = await request(`/annotations/${id}/replies?course_id=${courseId}`, {
      method: 'POST', body: JSON.stringify({ annotation: { content } }),
    });
    return data;
  },
  getAnnotationSummary: async (courseId, assignmentId, userId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/submissions/${userId}/annotation_summary`);
    return data;
  },

  // COPPA / Parental Consent
  requestConsent: async (userId) => {
    const { data } = await request('/consent/request', {
      method: 'POST', body: JSON.stringify({ user_id: userId }),
    });
    return data;
  },
  listConsents: async (page = 1, perPage = 20) => {
    return request(`/consent?page=${page}&per_page=${perPage}`);
  },
  verifyConsent: async (token, granted) => {
    const { data } = await request(`/consent/verify/${token}`, {
      method: 'POST', body: JSON.stringify({ granted }),
    });
    return data;
  },
  revokeConsent: async (id) => {
    const { data } = await request(`/consent/${id}`, { method: 'DELETE' });
    return data;
  },

  // FERPA
  createDataExportRequest: async (userId) => {
    const { data } = await request(`/users/${userId}/data_export`, { method: 'POST' });
    return data;
  },
  getDataExportRequest: async (userId, id) => {
    const { data } = await request(`/users/${userId}/data_export/${id}`);
    return data;
  },
  createDataDeletionRequest: async (userId, reason) => {
    const { data } = await request(`/users/${userId}/data_deletion`, {
      method: 'POST', body: JSON.stringify({ reason }),
    });
    return data;
  },
  getPendingDeletionRequests: async (page = 1, perPage = 20) => {
    return request(`/admin/data_deletion_requests?page=${page}&per_page=${perPage}`);
  },
  approveDeletionRequest: async (id) => {
    const { data } = await request(`/admin/data_deletion_requests/${id}/approve`, { method: 'POST' });
    return data;
  },
  getRetentionPolicies: async (page = 1, perPage = 20) => {
    return request(`/admin/retention_policies?page=${page}&per_page=${perPage}`);
  },
  createRetentionPolicy: async (policy) => {
    const { data } = await request('/admin/retention_policies', {
      method: 'POST', body: JSON.stringify({ policy }),
    });
    return data;
  },
  updateRetentionPolicy: async (id, policy) => {
    const { data } = await request(`/admin/retention_policies/${id}`, {
      method: 'PUT', body: JSON.stringify({ policy }),
    });
    return data;
  },
  deleteRetentionPolicy: async (id) => {
    const { data } = await request(`/admin/retention_policies/${id}`, { method: 'DELETE' });
    return data;
  },

  // Accommodations
  getUserAccommodations: async (userId, page = 1, perPage = 20) => {
    return request(`/users/${userId}/accommodations?page=${page}&per_page=${perPage}`);
  },
  createAccommodation: async (userId, accommodation) => {
    const { data } = await request(`/users/${userId}/accommodations`, {
      method: 'POST', body: JSON.stringify({ accommodation }),
    });
    return data;
  },
  getAccommodation: async (id) => {
    const { data } = await request(`/accommodations/${id}`);
    return data;
  },
  updateAccommodation: async (id, accommodation) => {
    const { data } = await request(`/accommodations/${id}`, {
      method: 'PUT', body: JSON.stringify({ accommodation }),
    });
    return data;
  },
  deleteAccommodation: async (id) => {
    const { data } = await request(`/accommodations/${id}`, { method: 'DELETE' });
    return data;
  },
  getCourseAccommodations: async (courseId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/accommodations?page=${page}&per_page=${perPage}`);
  },
  applyAccommodationsToAssignment: async (courseId, assignmentId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/apply_accommodations`, { method: 'POST' });
    return data;
  },

  // Attendance
  recordAttendance: async (courseId, attendance) => {
    const { data } = await request(`/courses/${courseId}/attendance`, {
      method: 'POST', body: JSON.stringify({ attendance }),
    });
    return data;
  },
  getClassAttendance: async (courseId, date) => {
    const params = date ? `?date=${date}` : '';
    const { data } = await request(`/courses/${courseId}/attendance${params}`);
    return data;
  },
  getStudentAttendance: async (courseId, userId, page = 1, perPage = 50) => {
    return request(`/courses/${courseId}/attendance/users/${userId}?page=${page}&per_page=${perPage}`);
  },
  getStudentAttendanceSummary: async (courseId, userId) => {
    const { data } = await request(`/courses/${courseId}/attendance/users/${userId}/summary`);
    return data;
  },
  exportAttendanceCSV: (courseId) => `${API_URL}/courses/${courseId}/attendance/export.csv`,

  // Portfolios
  listPortfolios: async (page = 1, perPage = 20) => {
    return request(`/users/self/portfolios?page=${page}&per_page=${perPage}`);
  },
  createPortfolio: async (portfolio) => {
    const { data } = await request('/users/self/portfolios', {
      method: 'POST', body: JSON.stringify({ portfolio }),
    });
    return data;
  },
  getPortfolio: async (id) => {
    const { data } = await request(`/portfolios/${id}`);
    return data;
  },
  updatePortfolio: async (id, portfolio) => {
    const { data } = await request(`/portfolios/${id}`, {
      method: 'PUT', body: JSON.stringify({ portfolio }),
    });
    return data;
  },
  deletePortfolio: async (id) => {
    const { data } = await request(`/portfolios/${id}`, { method: 'DELETE' });
    return data;
  },
  publishPortfolio: async (id) => {
    const { data } = await request(`/portfolios/${id}/publish`, { method: 'POST' });
    return data;
  },
  unpublishPortfolio: async (id) => {
    const { data } = await request(`/portfolios/${id}/publish`, { method: 'DELETE' });
    return data;
  },
  addPortfolioSection: async (id, section) => {
    const { data } = await request(`/portfolios/${id}/sections`, {
      method: 'POST', body: JSON.stringify({ section }),
    });
    return data;
  },
  updatePortfolioSection: async (id, sectionId, section) => {
    const { data } = await request(`/portfolios/${id}/sections/${sectionId}`, {
      method: 'PUT', body: JSON.stringify({ section }),
    });
    return data;
  },
  deletePortfolioSection: async (id, sectionId) => {
    const { data } = await request(`/portfolios/${id}/sections/${sectionId}`, { method: 'DELETE' });
    return data;
  },
  addPortfolioArtifact: async (id, artifact) => {
    const { data } = await request(`/portfolios/${id}/artifacts`, {
      method: 'POST', body: JSON.stringify({ artifact }),
    });
    return data;
  },
  updatePortfolioArtifact: async (id, artifactId, artifact) => {
    const { data } = await request(`/portfolios/${id}/artifacts/${artifactId}`, {
      method: 'PUT', body: JSON.stringify({ artifact }),
    });
    return data;
  },
  deletePortfolioArtifact: async (id, artifactId) => {
    const { data } = await request(`/portfolios/${id}/artifacts/${artifactId}`, { method: 'DELETE' });
    return data;
  },
  addPortfolioReflection: async (id, artifactId, reflection) => {
    const { data } = await request(`/portfolios/${id}/artifacts/${artifactId}/reflections`, {
      method: 'POST', body: JSON.stringify({ reflection }),
    });
    return data;
  },
  importPortfolioFromCourse: async (id, courseId) => {
    const { data } = await request(`/portfolios/${id}/import`, {
      method: 'POST', body: JSON.stringify({ course_id: courseId }),
    });
    return data;
  },
  exportPortfolioHTML: async (id) => {
    return requestRaw(`/portfolios/${id}/export/html`);
  },
  exportPortfolioPDF: async (id) => {
    return requestRaw(`/portfolios/${id}/export/pdf`);
  },
  getPortfolioComments: async (id, page = 1, perPage = 50) => {
    return request(`/portfolios/${id}/comments?page=${page}&per_page=${perPage}`);
  },
  addPortfolioComment: async (id, content) => {
    const { data } = await request(`/portfolios/${id}/comments`, {
      method: 'POST', body: JSON.stringify({ comment: { content } }),
    });
    return data;
  },
  getPortfolioTemplates: async (page = 1, perPage = 20) => {
    return request(`/portfolio_templates?page=${page}&per_page=${perPage}`);
  },
  createPortfolioFromTemplate: async (templateId, name) => {
    const { data } = await request(`/portfolio_templates/${templateId}/create`, {
      method: 'POST', body: JSON.stringify({ name }),
    });
    return data;
  },
  getPublicPortfolio: async (slug) => {
    const { data } = await request(`/portfolios/public/${slug}`);
    return data;
  },
  recordPortfolioView: async (slug) => {
    const { data } = await request(`/portfolios/public/${slug}`, { method: 'POST' });
    return data;
  },
  duplicatePortfolio: async (id) => {
    const { data } = await request(`/portfolios/${id}/import`, {
      method: 'POST', body: JSON.stringify({ duplicate: true }),
    });
    return data;
  },

  // SSO
  getSAMLLoginUrl: (providerId) => `${API_URL}/auth/saml/login?provider_id=${providerId}`,
  getSAMLMetadataUrl: () => `${API_URL}/auth/saml/metadata`,
  getCASLoginUrl: (providerId) => `${API_URL}/auth/cas/login?provider_id=${providerId}`,
  ldapLogin: async (providerId, username, password) => {
    const { data } = await request('/auth/ldap/login', {
      method: 'POST', body: JSON.stringify({ provider_id: providerId, username, password }),
    });
    return data;
  },

  // Course Home Engine
  getCourseHomeData: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/home`);
    return data;
  },
  recordCourseVisit: async (courseId, visit) => {
    const { data } = await request(`/courses/${courseId}/home/visit`, {
      method: 'POST', body: JSON.stringify(visit),
    });
    return data;
  },
  getCourseHomeButtons: async (courseId) => {
    return await request(`/courses/${courseId}/home/buttons`);
  },
  createCourseHomeButton: async (courseId, button) => {
    return await request(`/courses/${courseId}/home/buttons`, {
      method: 'POST', body: JSON.stringify(button),
    });
  },
  updateCourseHomeButton: async (courseId, buttonId, button) => {
    return await request(`/courses/${courseId}/home/buttons/${buttonId}`, {
      method: 'PUT', body: JSON.stringify(button),
    });
  },
  deleteCourseHomeButton: async (courseId, buttonId) => {
    const { data } = await request(`/courses/${courseId}/home/buttons/${buttonId}`, { method: 'DELETE' });
    return data;
  },
  reorderCourseHomeButtons: async (courseId, positions) => {
    const { data } = await request(`/courses/${courseId}/home/buttons/reorder`, {
      method: 'PUT', body: JSON.stringify({ positions }),
    });
    return data;
  },
  getTodaysLessonOverrides: async (courseId) => {
    return await request(`/courses/${courseId}/home/overrides`);
  },
  createTodaysLessonOverride: async (courseId, override) => {
    return await request(`/courses/${courseId}/home/overrides`, {
      method: 'POST', body: JSON.stringify(override),
    });
  },
  updateTodaysLessonOverride: async (courseId, overrideId, override) => {
    return await request(`/courses/${courseId}/home/overrides/${overrideId}`, {
      method: 'PUT', body: JSON.stringify(override),
    });
  },
  deleteTodaysLessonOverride: async (courseId, overrideId) => {
    const { data } = await request(`/courses/${courseId}/home/overrides/${overrideId}`, { method: 'DELETE' });
    return data;
  },

  // Peer Reviews
  assignPeerReviews: async (courseId, assignmentId, count = 1) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/peer_reviews`, {
      method: 'POST', body: JSON.stringify({ count }),
    });
    return data;
  },
  listPeerReviews: async (courseId, assignmentId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/peer_reviews`);
    return data || [];
  },
  listMyPeerReviews: async (courseId, assignmentId) => {
    const { data } = await request(`/courses/${courseId}/assignments/${assignmentId}/peer_reviews/mine`);
    return data || [];
  },
  submitPeerReview: async (reviewId, score, comments) => {
    const { data } = await request(`/peer_reviews/${reviewId}`, {
      method: 'PUT', body: JSON.stringify({ score, comments }),
    });
    return data;
  },

  // Question Banks
  listQuestionBanks: async (courseId, page = 1, perPage = 20) => {
    const { data } = await request(`/courses/${courseId}/question_banks?page=${page}&per_page=${perPage}`);
    return data || [];
  },
  createQuestionBank: async (courseId, title) => {
    const { data } = await request(`/courses/${courseId}/question_banks`, {
      method: 'POST', body: JSON.stringify({ title }),
    });
    return data;
  },
  getQuestionBank: async (courseId, bankId) => {
    const { data } = await request(`/courses/${courseId}/question_banks/${bankId}`);
    return data;
  },
  updateQuestionBank: async (courseId, bankId, title) => {
    const { data } = await request(`/courses/${courseId}/question_banks/${bankId}`, {
      method: 'PUT', body: JSON.stringify({ title }),
    });
    return data;
  },
  deleteQuestionBank: async (courseId, bankId) => {
    const { data } = await request(`/courses/${courseId}/question_banks/${bankId}`, { method: 'DELETE' });
    return data;
  },
  listBankQuestions: async (courseId, bankId) => {
    const { data } = await request(`/courses/${courseId}/question_banks/${bankId}/questions`);
    return data || [];
  },
  addBankQuestion: async (courseId, bankId, question) => {
    const { data } = await request(`/courses/${courseId}/question_banks/${bankId}/questions`, {
      method: 'POST', body: JSON.stringify(question),
    });
    return data;
  },
  updateBankQuestion: async (courseId, bankId, questionId, question) => {
    const { data } = await request(`/courses/${courseId}/question_banks/${bankId}/questions/${questionId}`, {
      method: 'PUT', body: JSON.stringify(question),
    });
    return data;
  },
  deleteBankQuestion: async (courseId, bankId, questionId) => {
    const { data } = await request(`/courses/${courseId}/question_banks/${bankId}/questions/${questionId}`, { method: 'DELETE' });
    return data;
  },
  pullBankQuestionsToQuiz: async (courseId, bankId, quizId, questionIds = []) => {
    const { data } = await request(`/courses/${courseId}/question_banks/${bankId}/pull_to_quiz`, {
      method: 'POST', body: JSON.stringify({ quiz_id: quizId, question_ids: questionIds }),
    });
    return data;
  },

  // ==========================================================================
  // Wave A: Unified Quiz Engine — Item banks, stimuli, item analysis
  // ==========================================================================
  // Add a single bank item to a quiz.
  addBankItemToQuiz: async (bankId, itemId, quizId) => {
    const { data } = await request(`/quiz_item_banks/${bankId}/items/${itemId}/add_to_quiz/${quizId}`, {
      method: 'POST',
    });
    return data;
  },
  // Random draw: copy N random items from a bank into a quiz.
  randomDrawFromBank: async (bankId, quizId, count) => {
    const { data } = await request(`/quiz_item_banks/${bankId}/random_draw`, {
      method: 'POST', body: JSON.stringify({ quiz_id: quizId, count }),
    });
    return data;
  },

  // Stimulus passages
  listStimuli: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/quiz_stimuli`);
    return data || [];
  },
  getStimulus: async (courseId, stimulusId) => {
    const { data } = await request(`/courses/${courseId}/quiz_stimuli/${stimulusId}`);
    return data;
  },
  createStimulus: async (courseId, payload) => {
    const { data } = await request(`/courses/${courseId}/quiz_stimuli`, {
      method: 'POST', body: JSON.stringify(payload),
    });
    return data;
  },
  updateStimulus: async (courseId, stimulusId, payload) => {
    const { data } = await request(`/courses/${courseId}/quiz_stimuli/${stimulusId}`, {
      method: 'PUT', body: JSON.stringify(payload),
    });
    return data;
  },
  deleteStimulus: async (courseId, stimulusId) => {
    const { data } = await request(`/courses/${courseId}/quiz_stimuli/${stimulusId}`, { method: 'DELETE' });
    return data;
  },
  getStimulusQuestions: async (stimulusId) => {
    const { data } = await request(`/quiz_stimuli/${stimulusId}/questions`);
    return data || [];
  },

  // Item analysis — no dedicated backend endpoint yet. ItemAnalysisPage falls
  // back to client-side aggregation over submissions; returning null here makes
  // the fallback path engage cleanly without a 404 round-trip.
  getQuizItemAnalysis: async (_courseId, _quizId) => {
    return null;
  },

  // Module Prerequisites
  getModulePrerequisites: async (courseId, moduleId) => {
    const { data } = await request(`/courses/${courseId}/modules/${moduleId}/prerequisites`);
    return data;
  },
  setModulePrerequisites: async (courseId, moduleId, prerequisiteModuleIds) => {
    const { data } = await request(`/courses/${courseId}/modules/${moduleId}/prerequisites`, {
      method: 'PUT', body: JSON.stringify({ prerequisite_module_ids: prerequisiteModuleIds }),
    });
    return data;
  },

  // ==========================================================================
  // P3 Features: Feature Flags
  // ==========================================================================
  listAccountFeatureFlags: (accountId) =>
    request(`/accounts/${accountId}/features`),
  setAccountFeatureFlag: (accountId, feature, state) =>
    request(`/accounts/${accountId}/features/${feature}`, {
      method: 'PUT',
      body: JSON.stringify({ state }),
    }),
  resetAccountFeatureFlag: (accountId, feature) =>
    request(`/accounts/${accountId}/features/${feature}`, { method: 'DELETE' }),
  listCourseFeatureFlags: (courseId) =>
    request(`/courses/${courseId}/features`),
  setCourseFeatureFlag: (courseId, feature, state) =>
    request(`/courses/${courseId}/features/${feature}`, {
      method: 'PUT',
      body: JSON.stringify({ state }),
    }),
  listUserFeatureFlags: () => request(`/users/self/features`),

  // ==========================================================================
  // P3 Features: Custom Gradebook Columns
  // ==========================================================================
  listCustomGradebookColumns: (courseId, includeHidden = false) =>
    request(`/courses/${courseId}/custom_gradebook_columns?include_hidden=${includeHidden}`),
  createCustomGradebookColumn: (courseId, data) =>
    request(`/courses/${courseId}/custom_gradebook_columns`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  updateCustomGradebookColumn: (courseId, columnId, data) =>
    request(`/courses/${courseId}/custom_gradebook_columns/${columnId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  deleteCustomGradebookColumn: (courseId, columnId) =>
    request(`/courses/${courseId}/custom_gradebook_columns/${columnId}`, {
      method: 'DELETE',
    }),
  reorderCustomGradebookColumns: (courseId, order) =>
    request(`/courses/${courseId}/custom_gradebook_columns/reorder`, {
      method: 'POST',
      body: JSON.stringify({ order }),
    }),
  getCustomColumnData: (courseId, columnId) =>
    request(`/courses/${courseId}/custom_gradebook_columns/${columnId}/data`),
  setCustomColumnCell: (courseId, columnId, userId, content) =>
    request(`/courses/${courseId}/custom_gradebook_columns/${columnId}/data/${userId}`, {
      method: 'PUT',
      body: JSON.stringify({ content }),
    }),
  bulkUpdateCustomColumns: (courseId, entries) =>
    request(`/courses/${courseId}/custom_gradebook_columns/data`, {
      method: 'PUT',
      body: JSON.stringify({ entries }),
    }),

  // ==========================================================================
  // P3 Features: Mastery Paths (Conditional Release)
  // ==========================================================================
  listMasteryPathRules: (courseId) =>
    request(`/courses/${courseId}/mastery_paths/rules`),
  getMasteryPathRule: (courseId, assignmentId) =>
    request(`/courses/${courseId}/mastery_paths/rules/${assignmentId}`),
  createMasteryPathRule: (courseId, payload) =>
    request(`/courses/${courseId}/mastery_paths/rules`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  updateMasteryPathRule: (courseId, ruleId, payload) =>
    request(`/courses/${courseId}/mastery_paths/rules/${ruleId}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),
  deleteMasteryPathRule: (courseId, ruleId) =>
    request(`/courses/${courseId}/mastery_paths/rules/${ruleId}`, {
      method: 'DELETE',
    }),

  // ==========================================================================
  // P3 Features: Appointment Groups (Scheduler)
  // ==========================================================================
  listAppointmentGroups: (courseId) =>
    request(`/courses/${courseId}/appointment_groups`),
  getAppointmentGroup: (id) =>
    request(`/appointment_groups/${id}`),
  createAppointmentGroup: (courseId, data) =>
    request(`/courses/${courseId}/appointment_groups`, { method: 'POST', body: JSON.stringify(data) }),
  updateAppointmentGroup: (id, data) =>
    request(`/appointment_groups/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteAppointmentGroup: (id) =>
    request(`/appointment_groups/${id}`, { method: 'DELETE' }),
  listAppointmentSlots: (id, includeFull = false) =>
    request(`/appointment_groups/${id}/appointments${includeFull ? '?include_full=true' : ''}`),
  listAppointmentReservations: (id, slotId) =>
    request(`/appointment_groups/${id}/appointments/${slotId}/reservations`),
  reserveAppointmentSlot: (id, slotId) =>
    request(`/appointment_groups/${id}/appointments/${slotId}/reservations`, { method: 'POST', body: '{}' }),
  cancelAppointmentReservation: (id, slotId, reservationId) =>
    request(`/appointment_groups/${id}/appointments/${slotId}/reservations/${reservationId}`, { method: 'DELETE' }),
  listMyAppointmentReservations: () =>
    request(`/users/self/appointments`).catch(() => ({ data: [] })),

  // ==========================================================================
  // P3 Features: Outcome Proficiency + Learning Mastery Gradebook
  // ==========================================================================
  getAccountOutcomeProficiency: (accountId) =>
    request(`/accounts/${accountId}/outcome_proficiency`),
  setAccountOutcomeProficiency: (accountId, data) =>
    request(`/accounts/${accountId}/outcome_proficiency`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  deleteAccountOutcomeProficiency: (accountId) =>
    request(`/accounts/${accountId}/outcome_proficiency`, { method: 'DELETE' }),
  getCourseOutcomeProficiency: (courseId) =>
    request(`/courses/${courseId}/outcome_proficiency`),
  setCourseOutcomeProficiency: (courseId, data) =>
    request(`/courses/${courseId}/outcome_proficiency`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  deleteCourseOutcomeProficiency: (courseId) =>
    request(`/courses/${courseId}/outcome_proficiency`, { method: 'DELETE' }),
  getLearningMasteryGradebook: (courseId) =>
    request(`/courses/${courseId}/learning_mastery_gradebook`),

  // Discussion Checkpoints
  getDiscussionCheckpoints: (courseId, topicId) =>
    request(`/courses/${courseId}/discussion_topics/${topicId}/checkpoints`),
  createDiscussionCheckpoints: (courseId, topicId, body) =>
    request(`/courses/${courseId}/discussion_topics/${topicId}/checkpoints`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  updateDiscussionCheckpoint: (courseId, topicId, id, body) =>
    request(`/courses/${courseId}/discussion_topics/${topicId}/checkpoints/${id}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  deleteDiscussionCheckpoint: (courseId, topicId, id) =>
    request(`/courses/${courseId}/discussion_topics/${topicId}/checkpoints/${id}`, {
      method: 'DELETE',
    }),
  getDiscussionCheckpointProgress: (courseId, topicId, userId) =>
    request(`/courses/${courseId}/discussion_topics/${topicId}/checkpoints/progress?user_id=${userId}`),

  // Smart Search (pgvector cosine similarity)
  smartSearch: (courseId, q, limit = 10) =>
    request(`/courses/${courseId}/smart_search?q=${encodeURIComponent(q)}&limit=${limit}`),
  reindexSmartSearch: (courseId) =>
    request(`/courses/${courseId}/smart_search/reindex`, { method: 'POST' }),

  // Commons content library
  browseCommons: (params = {}) =>
    request(`/commons?${new URLSearchParams(params).toString()}`),
  getCommonsItem: (id) => request(`/commons/${id}`),
  listCommonsFavorites: () => request('/commons/favorites'),
  toggleCommonsFavorite: (id) =>
    request(`/commons/${id}/favorite`, { method: 'POST' }),
  importCommons: (id, courseId) =>
    request(`/commons/${id}/import?course_id=${courseId}`, { method: 'POST' }),
  publishCommons: (courseId, body) =>
    request(`/courses/${courseId}/commons/publish`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
};
