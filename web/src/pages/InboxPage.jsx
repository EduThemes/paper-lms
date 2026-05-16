import React, { useState, useEffect, useCallback, useRef } from 'react';
import { Mail, Send, Plus, MessageSquare, X, Search, ArrowLeft, Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';
import { Skeleton } from '@/components/ui/skeleton';

const InboxPage = () => {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [conversations, setConversations] = useState([]);
  const [selectedConv, setSelectedConv] = useState(null);
  const [messages, setMessages] = useState([]);
  const [loading, setLoading] = useState(true);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [error, setError] = useState(null);
  const [replyText, setReplyText] = useState('');
  const [sending, setSending] = useState(false);
  const [showNewForm, setShowNewForm] = useState(false);
  const [newSubject, setNewSubject] = useState('');
  const [newBody, setNewBody] = useState('');
  const [creating, setCreating] = useState(false);

  // Recipient picker state
  const [selectedRecipients, setSelectedRecipients] = useState([]);
  const [recipientSearch, setRecipientSearch] = useState('');
  const [searchResults, setSearchResults] = useState([]);
  const [searching, setSearching] = useState(false);
  const [showDropdown, setShowDropdown] = useState(false);
  const searchRef = useRef(null);
  const dropdownRef = useRef(null);
  const searchTimerRef = useRef(null);

  const fetchConversations = useCallback(async () => {
    try {
      const { data } = await api.getConversations();
      setConversations(data);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchConversations();
  }, [fetchConversations]);

  // Debounced user search
  useEffect(() => {
    if (recipientSearch.trim().length < 2) {
      setSearchResults([]);
      setShowDropdown(false);
      return;
    }

    if (searchTimerRef.current) clearTimeout(searchTimerRef.current);
    searchTimerRef.current = setTimeout(async () => {
      setSearching(true);
      try {
        const result = await api.searchUsers(recipientSearch.trim(), 1, 8);
        const users = Array.isArray(result) ? result : (result.data || []);
        // Filter out already-selected recipients and self
        const filtered = users.filter(
          (u) => u.id !== user?.id && !selectedRecipients.some((r) => r.id === u.id)
        );
        setSearchResults(filtered);
        setShowDropdown(filtered.length > 0);
      } catch {
        setSearchResults([]);
      } finally {
        setSearching(false);
      }
    }, 300);

    return () => { if (searchTimerRef.current) clearTimeout(searchTimerRef.current); };
  }, [recipientSearch, selectedRecipients, user?.id]);

  // Close dropdown on outside click
  useEffect(() => {
    const handleClickOutside = (e) => {
      if (
        dropdownRef.current && !dropdownRef.current.contains(e.target) &&
        searchRef.current && !searchRef.current.contains(e.target)
      ) {
        setShowDropdown(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const addRecipient = (u) => {
    setSelectedRecipients((prev) => [...prev, u]);
    setRecipientSearch('');
    setSearchResults([]);
    setShowDropdown(false);
    searchRef.current?.focus();
  };

  const removeRecipient = (id) => {
    setSelectedRecipients((prev) => prev.filter((r) => r.id !== id));
  };

  const selectConversation = async (conv) => {
    setSelectedConv(conv);
    setMessagesLoading(true);
    setError(null);
    try {
      const { data } = await api.getConversationMessages(conv.id);
      setMessages(data);
      await api.markConversationAsRead(conv.id);
    } catch (err) {
      setError(err.message);
    } finally {
      setMessagesLoading(false);
    }
  };

  const handleSendReply = async (e) => {
    e.preventDefault();
    if (!replyText.trim() || !selectedConv) return;
    setSending(true);
    try {
      const newMsg = await api.createConversationMessage(selectedConv.id, replyText);
      setMessages((prev) => [...prev, newMsg]);
      setReplyText('');
      fetchConversations();
    } catch (err) {
      setError(err.message);
    } finally {
      setSending(false);
    }
  };

  const handleCreateConversation = async (e) => {
    e.preventDefault();
    if (selectedRecipients.length === 0) return;
    setCreating(true);
    try {
      const recipientIDs = selectedRecipients.map((r) => r.id);
      const conv = await api.createConversation({
        subject: newSubject,
        recipients: recipientIDs,
      });

      if (newBody.trim()) {
        await api.createConversationMessage(conv.id, newBody);
      }

      setNewSubject('');
      setNewBody('');
      setSelectedRecipients([]);
      setShowNewForm(false);
      setLoading(true);
      await fetchConversations();
      setSelectedConv(conv);
      if (newBody.trim()) {
        const { data } = await api.getConversationMessages(conv.id);
        setMessages(data);
      } else {
        setMessages([]);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const resetNewForm = () => {
    setShowNewForm(false);
    setNewSubject('');
    setNewBody('');
    setSelectedRecipients([]);
    setRecipientSearch('');
    setSearchResults([]);
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now - date;
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

    if (diffDays === 0) {
      return date.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
    }
    if (diffDays < 7) {
      return date.toLocaleDateString(undefined, { weekday: 'short' });
    }
    return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  };

  if (loading) {
    return (
      <Layout>
        <div className="space-y-3 p-6">
          <Skeleton className="h-9 w-48" />
          <Skeleton className="h-12 w-full" />
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-2xl font-bold text-text-primary flex items-center gap-2">
          <Mail className="w-6 h-6" />
          {t('inboxPage.title')}
        </h2>
        <button
          onClick={() => {
            if (showNewForm) resetNewForm();
            else setShowNewForm(true);
          }}
          className="inline-flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
          aria-label={t('inboxPage.compose')}
        >
          {showNewForm ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
          <span>{showNewForm ? t('common.cancel') : t('inboxPage.newMessage')}</span>
        </button>
      </div>

      {error && (
        <div className="bg-accent-danger/10 text-accent-danger rounded-md p-3 mb-4 text-sm flex items-center justify-between">
          <span>{error}</span>
          <button
            onClick={() => { setError(null); setLoading(true); fetchConversations(); }}
            className="ml-3 text-accent-danger hover:text-red-900 font-medium underline text-sm flex-shrink-0"
          >
            Retry
          </button>
        </div>
      )}

      {showNewForm && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">{t('inboxPage.newMessage')}</h3>
          <form onSubmit={handleCreateConversation} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">{t('inboxPage.to')}</label>
              <div className="border border-border-strong rounded-md px-2 py-1.5 focus-within:ring-2 focus-within:ring-brand-500 focus-within:border-brand-500">
                <div className="flex flex-wrap gap-1.5 items-center">
                  {selectedRecipients.map((r) => (
                    <span
                      key={r.id}
                      className="inline-flex items-center gap-1 bg-brand-100 text-brand-800 text-xs font-medium px-2 py-1 rounded-full"
                    >
                      {r.name}
                      <button
                        type="button"
                        onClick={() => removeRecipient(r.id)}
                        className="text-brand-600 hover:text-brand-800"
                      >
                        <X className="w-3 h-3" />
                      </button>
                    </span>
                  ))}
                  <div className="relative flex-1 min-w-[150px]">
                    <input
                      ref={searchRef}
                      type="text"
                      value={recipientSearch}
                      onChange={(e) => setRecipientSearch(e.target.value)}
                      onFocus={() => { if (searchResults.length > 0) setShowDropdown(true); }}
                      onKeyDown={(e) => {
                        if (e.key === 'Escape') {
                          e.stopPropagation();
                          setShowDropdown(false);
                        }
                      }}
                      className="w-full border-0 px-1 py-0.5 text-sm focus:outline-none focus:ring-0"
                      placeholder={selectedRecipients.length === 0 ? 'Search by name or email...' : 'Add more...'}
                    />
                    {showDropdown && (
                      <div
                        ref={dropdownRef}
                        className="absolute z-10 mt-1 w-full bg-surface-0 border border-border-default rounded-md shadow-lg max-h-48 overflow-y-auto"
                      >
                        {searching ? (
                          <div className="px-3 py-2 text-sm text-text-tertiary">Searching...</div>
                        ) : (
                          searchResults.map((u) => (
                            <button
                              key={u.id}
                              type="button"
                              onMouseDown={(e) => e.preventDefault()}
                              onClick={() => addRecipient(u)}
                              className="w-full text-left px-3 py-2 hover:bg-brand-50 flex items-center gap-2 text-sm"
                            >
                              <div className="w-7 h-7 rounded-full bg-border-default flex items-center justify-center text-xs font-medium text-text-secondary flex-shrink-0">
                                {(u.name || '?')[0].toUpperCase()}
                              </div>
                              <div className="min-w-0">
                                <p className="font-medium text-text-primary truncate">{u.name}</p>
                                <p className="text-xs text-text-tertiary truncate">{u.email}</p>
                              </div>
                            </button>
                          ))
                        )}
                      </div>
                    )}
                  </div>
                </div>
              </div>
              {selectedRecipients.length === 0 && (
                <p className="text-xs text-text-tertiary mt-1">Type at least 2 characters to search</p>
              )}
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">{t('inboxPage.subject')}</label>
              <input
                type="text"
                value={newSubject}
                onChange={(e) => setNewSubject(e.target.value)}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                required
                placeholder="Conversation subject"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">{t('inboxPage.message')}</label>
              <textarea
                value={newBody}
                onChange={(e) => setNewBody(e.target.value)}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                rows={3}
                placeholder="Write your message..."
              />
            </div>
            <div className="flex justify-end">
              <button
                type="submit"
                disabled={creating || selectedRecipients.length === 0}
                className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
              >
                {creating ? t('inboxPage.sending') : t('inboxPage.send')}
              </button>
            </div>
          </form>
        </div>
      )}

      <div className="flex flex-col md:flex-row bg-surface-0 rounded-lg shadow overflow-hidden" style={{ minHeight: '500px' }}>
        {/* Left panel - conversation list */}
        <div className={`w-full md:w-1/3 border-r border-border-default overflow-y-auto ${selectedConv ? 'hidden md:block' : 'block'}`}>
          {conversations.length === 0 ? (
            <div className="p-6 text-center text-text-tertiary text-sm">
              <MessageSquare className="w-8 h-8 mx-auto mb-2 text-gray-300" />
              No conversations yet.
            </div>
          ) : (
            <div className="divide-y divide-gray-100">
              {conversations.map((conv) => (
                <button
                  key={conv.id}
                  onClick={() => selectConversation(conv)}
                  className={`w-full text-left p-4 hover:bg-surface-1 transition-colors ${
                    selectedConv?.id === conv.id ? 'bg-brand-50 border-l-2 border-brand-600' : ''
                  }`}
                >
                  <div className="flex items-start justify-between">
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium text-text-primary truncate">
                        {conv.subject || '(No subject)'}
                      </p>
                      <p className="text-xs text-text-tertiary mt-1">
                        {conv.participants?.length
                          ? conv.participants.map((p) => p.name || `User #${p.id}`).join(', ')
                          : `From user #${conv.created_by_user_id}`}
                      </p>
                    </div>
                    <span className="text-xs text-text-disabled ml-2 flex-shrink-0">
                      {formatDate(conv.last_message_at)}
                    </span>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Right panel - message thread */}
        <div className={`flex-1 ${!selectedConv ? 'hidden md:flex' : 'flex'} flex-col`}>
          {selectedConv ? (
            <>
              {/* Thread header */}
              <div className="p-4 border-b border-border-default bg-surface-1">
                <button
                  onClick={() => setSelectedConv(null)}
                  className="md:hidden flex items-center text-sm text-brand-600 hover:text-brand-800 mb-2"
                >
                  <ArrowLeft className="w-4 h-4 mr-1" />
                  Back to conversations
                </button>
                <h3 className="font-semibold text-text-primary">
                  {selectedConv.subject || '(No subject)'}
                </h3>
                <p className="text-xs text-text-tertiary mt-1">
                  {selectedConv.participants?.length
                    ? selectedConv.participants.map((p) => p.name || `User #${p.id}`).join(', ')
                    : `Started by user #${selectedConv.created_by_user_id}`}
                </p>
              </div>

              {/* Messages */}
              <div className="flex-1 overflow-y-auto p-4 space-y-4">
                {messagesLoading ? (
                  <div className="flex items-center justify-center text-text-tertiary py-8 text-sm">
                    <Loader2 className="w-5 h-5 animate-spin mr-2" />
                    <span>Loading messages...</span>
                  </div>
                ) : messages.length === 0 ? (
                  <div className="text-center text-text-disabled py-8 text-sm">
                    No messages yet. Start the conversation!
                  </div>
                ) : (
                  messages.map((msg) => {
                    const isOwn = msg.user_id === user?.id;
                    return (
                      <div
                        key={msg.id}
                        className={`flex ${isOwn ? 'justify-end' : 'justify-start'}`}
                      >
                        <div
                          className={`max-w-[70%] rounded-lg px-4 py-2 ${
                            isOwn
                              ? 'bg-brand-600 text-white'
                              : 'bg-surface-2 text-text-primary'
                          }`}
                        >
                          {!isOwn && (
                            <p className="text-xs font-medium text-text-tertiary mb-1">
                              {msg.user_name || msg.author_name || `User #${msg.user_id}`}
                            </p>
                          )}
                          <p className="text-sm whitespace-pre-wrap">{msg.body}</p>
                          <p
                            className={`text-xs mt-1 ${
                              isOwn ? 'text-blue-200' : 'text-text-disabled'
                            }`}
                          >
                            {formatDate(msg.created_at)}
                          </p>
                        </div>
                      </div>
                    );
                  })
                )}
              </div>

              {/* Reply area */}
              <div className="border-t border-border-default p-4">
                <form onSubmit={handleSendReply} className="flex items-end space-x-2">
                  <textarea
                    value={replyText}
                    onChange={(e) => setReplyText(e.target.value)}
                    placeholder="Type a message..."
                    className="flex-1 border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 resize-none"
                    rows={2}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault();
                        handleSendReply(e);
                      }
                    }}
                  />
                  <button
                    type="submit"
                    disabled={sending || !replyText.trim()}
                    className="bg-brand-600 text-white p-2 rounded-md hover:bg-brand-700 disabled:opacity-50"
                    title="Send message"
                  >
                    <Send className="w-5 h-5" />
                  </button>
                </form>
              </div>
            </>
          ) : (
            <div className="flex-1 flex flex-col items-center justify-center text-text-disabled">
              <Mail className="w-12 h-12 mb-3" />
              <p className="text-sm">Select a conversation to view messages</p>
            </div>
          )}
        </div>
      </div>
    </Layout>
  );
};

export default InboxPage;
