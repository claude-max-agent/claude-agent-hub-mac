import React, { useState } from 'react';
import { useTheme } from '../../theme';
import { useIsMobile } from '../../hooks/useIsMobile';
import { useModal } from '../../contexts/ModalContext';
import { api } from '../../api/client';

interface Props {
  onClose: () => void;
}

export const NotificationModal: React.FC<Props> = ({ onClose }) => {
  const isMobile = useIsMobile();
  const { colors, isDark } = useTheme();
  const { showAlert } = useModal();
  const [notification, setNotification] = useState({ title: '', message: '', isAlert: false });

  const handleSend = async () => {
    if (!notification.title.trim() || !notification.message.trim()) return;
    try {
      await api.sendNotification(notification.title, notification.message, notification.isAlert);
      onClose();
      showAlert('Notification sent!');
    } catch (err) {
      console.error('Failed to send notification:', err);
    }
  };

  return (
    <div style={{ position: 'fixed', inset: 0, backgroundColor: isDark ? 'rgba(0,0,0,0.7)' : 'rgba(0,0,0,0.5)', display: 'flex', alignItems: isMobile ? 'flex-end' : 'center', justifyContent: 'center', zIndex: 200 }} onClick={onClose}>
      <div style={{ backgroundColor: colors.cardBg, borderRadius: isMobile ? '16px 16px 0 0' : '8px', padding: '20px', width: isMobile ? '100%' : '400px', maxWidth: isMobile ? '100%' : '90%', paddingBottom: isMobile ? 'max(20px, env(safe-area-inset-bottom))' : '20px' }} onClick={e => e.stopPropagation()}>
        <h3 style={{ margin: '0 0 16px', fontSize: '18px', color: colors.text }}>Send Notification</h3>
        <input
          style={{ width: '100%', padding: '8px 12px', borderRadius: '6px', border: `1px solid ${colors.inputBorder}`, backgroundColor: colors.inputBg, color: colors.text, marginBottom: '12px', fontSize: '14px', boxSizing: 'border-box' }}
          placeholder="Title"
          value={notification.title}
          onChange={(e) => setNotification({ ...notification, title: e.target.value })}
        />
        <textarea
          style={{ width: '100%', padding: '8px 12px', borderRadius: '6px', border: `1px solid ${colors.inputBorder}`, backgroundColor: colors.inputBg, color: colors.text, marginBottom: '12px', fontSize: '14px', minHeight: '80px', resize: 'vertical', boxSizing: 'border-box' }}
          placeholder="Message..."
          value={notification.message}
          onChange={(e) => setNotification({ ...notification, message: e.target.value })}
        />
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '16px' }}>
          <input type="checkbox" id="isAlert" checked={notification.isAlert} onChange={(e) => setNotification({ ...notification, isAlert: e.target.checked })} />
          <label htmlFor="isAlert" style={{ fontSize: '14px', color: colors.text }}>Send as Alert</label>
        </div>
        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
          <button onClick={onClose} style={{ padding: '8px 16px', borderRadius: '6px', border: `1px solid ${colors.inputBorder}`, backgroundColor: colors.inputBg, color: colors.text, cursor: 'pointer' }}>Cancel</button>
          <button onClick={handleSend} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', backgroundColor: notification.isAlert ? '#ef4444' : '#3b82f6', color: '#fff', cursor: 'pointer' }}>Send</button>
        </div>
      </div>
    </div>
  );
};
