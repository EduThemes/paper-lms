// React Query bindings for api_notify_inbox.js (inbox, address book,
// announcement replies, notification policy grid).

import { useQuery, useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { inboxApi, announcementRepliesApi, notificationPoliciesApi } from './api_notify_inbox';
import { pageFromLink } from '../utils/notifyInbox';

export const notifyInboxKeys = {
  inbox: ['inbox'],
  conversations: (params) => ['inbox', 'conversations', params],
  conversation: (id) => ['inbox', 'conversation', String(id)],
  messages: (id) => ['inbox', 'messages', String(id)],
  unread: ['inbox', 'unread'],
  contexts: ['inbox', 'contexts'],
  recipients: (params) => ['inbox', 'recipients', params],
  replies: (announcementId) => ['announcementReplies', String(announcementId)],
  policies: ['notificationPolicies'],
};

const CONVERSATIONS_PER_PAGE = 20;

// Conversation list with "load more" pages.
export function useConversations({ scope = 'inbox', filter = '', search = '' } = {}) {
  return useInfiniteQuery({
    queryKey: notifyInboxKeys.conversations({ scope, filter, search }),
    queryFn: ({ pageParam = 1 }) =>
      inboxApi.listConversations({ scope, filter, search, page: pageParam, perPage: CONVERSATIONS_PER_PAGE }),
    initialPageParam: 1,
    getNextPageParam: (last) => pageFromLink(last?.pagination?.next) || undefined,
  });
}

export function useUnreadCount(options = {}) {
  return useQuery({
    queryKey: notifyInboxKeys.unread,
    queryFn: inboxApi.unreadCount,
    refetchInterval: 60_000,
    ...options,
  });
}

export function useInboxContexts() {
  return useQuery({ queryKey: notifyInboxKeys.contexts, queryFn: inboxApi.contexts, staleTime: 5 * 60_000 });
}

export function useRecipientSearch({ search = '', context = '' }, options = {}) {
  const term = search.trim();
  return useQuery({
    queryKey: notifyInboxKeys.recipients({ term, context }),
    queryFn: () => inboxApi.searchRecipients({ search: term, context, perPage: 20 }),
    enabled: !!context || term.length >= 2,
    staleTime: 30_000,
    ...options,
  });
}

// Invalidates every inbox list and the unread badge.
export function useInvalidateInbox() {
  const qc = useQueryClient();
  return () => {
    qc.invalidateQueries({ queryKey: ['inbox', 'conversations'] });
    qc.invalidateQueries({ queryKey: notifyInboxKeys.unread });
  };
}

export function useAnnouncementReplies(announcementId, options = {}) {
  return useQuery({
    queryKey: notifyInboxKeys.replies(announcementId),
    queryFn: () => announcementRepliesApi.list(announcementId),
    enabled: !!announcementId,
    ...options,
  });
}

function useRepliesMutation(announcementId, mutationFn, onDone) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: (...args) => {
      qc.invalidateQueries({ queryKey: notifyInboxKeys.replies(announcementId) });
      onDone?.(...args);
    },
  });
}

export function useCreateAnnouncementReply(announcementId, onDone) {
  return useRepliesMutation(
    announcementId,
    ({ message, parentId }) => announcementRepliesApi.create(announcementId, message, parentId),
    onDone,
  );
}

export function useUpdateAnnouncementReply(announcementId, onDone) {
  return useRepliesMutation(
    announcementId,
    ({ replyId, message }) => announcementRepliesApi.update(announcementId, replyId, message),
    onDone,
  );
}

export function useDeleteAnnouncementReply(announcementId, onDone) {
  return useRepliesMutation(announcementId, (replyId) => announcementRepliesApi.remove(announcementId, replyId), onDone);
}

export function useNotificationPolicies() {
  return useQuery({ queryKey: notifyInboxKeys.policies, queryFn: notificationPoliciesApi.get });
}

export function useUpdateNotificationPolicies(options = {}) {
  const qc = useQueryClient();
  const { onSuccess, ...rest } = options;
  return useMutation({
    mutationFn: (policies) => notificationPoliciesApi.update(policies),
    ...rest,
    onSuccess: (grid, ...args) => {
      qc.setQueryData(notifyInboxKeys.policies, grid);
      onSuccess?.(grid, ...args);
    },
  });
}
