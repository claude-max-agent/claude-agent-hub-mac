import React, { createContext, useContext, useState, useCallback } from 'react';
import type { ConfirmModalState, ThemeColors } from '../types';
import { useTheme } from '../theme';

interface ModalContextType {
  showConfirm: (state: ConfirmModalState) => void;
  hideConfirm: () => void;
  showAlert: (message: string) => void;
  hideAlert: () => void;
}

const ModalContext = createContext<ModalContextType>(null!);

export const useModal = () => useContext(ModalContext);

export const ModalProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { colors, isDark } = useTheme();
  const [confirmModal, setConfirmModal] = useState<ConfirmModalState | null>(null);
  const [alertModal, setAlertModal] = useState<string | null>(null);

  const showConfirm = useCallback((state: ConfirmModalState) => setConfirmModal(state), []);
  const hideConfirm = useCallback(() => setConfirmModal(null), []);
  const showAlert = useCallback((message: string) => setAlertModal(message), []);
  const hideAlert = useCallback(() => setAlertModal(null), []);

  return (
    <ModalContext.Provider value={{ showConfirm, hideConfirm, showAlert, hideAlert }}>
      {children}
      {confirmModal && <ConfirmModalView state={confirmModal} colors={colors} isDark={isDark} onClose={hideConfirm} />}
      {alertModal && <AlertModalView message={alertModal} colors={colors} isDark={isDark} onClose={hideAlert} />}
    </ModalContext.Provider>
  );
};

const ConfirmModalView: React.FC<{ state: ConfirmModalState; colors: ThemeColors; isDark: boolean; onClose: () => void }> = ({ state, colors, isDark, onClose }) => {
  const handleConfirm = async () => {
    try {
      await state.onConfirm();
    } finally {
      onClose();
    }
  };

  return (
    <div style={{ position: 'fixed', inset: 0, backgroundColor: isDark ? 'rgba(0,0,0,0.7)' : 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 300 }} onClick={onClose}>
      <div style={{ backgroundColor: colors.cardBg, borderRadius: '8px', padding: '20px', width: '360px', maxWidth: '90%' }} onClick={e => e.stopPropagation()}>
        <div style={{ fontSize: '14px', color: colors.text, marginBottom: '20px', lineHeight: '1.5' }}>
          {state.message}
        </div>
        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
          <button
            onClick={onClose}
            style={{ padding: '8px 16px', borderRadius: '6px', border: `1px solid ${colors.inputBorder}`, backgroundColor: colors.inputBg, color: colors.text, cursor: 'pointer' }}
          >
            {state.cancelText || 'Cancel'}
          </button>
          <button
            onClick={handleConfirm}
            style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', backgroundColor: state.isDanger ? '#ef4444' : '#3b82f6', color: '#fff', cursor: 'pointer' }}
          >
            {state.confirmText || 'OK'}
          </button>
        </div>
      </div>
    </div>
  );
};

const AlertModalView: React.FC<{ message: string; colors: ThemeColors; isDark: boolean; onClose: () => void }> = ({ message, colors, isDark, onClose }) => (
  <div style={{ position: 'fixed', inset: 0, backgroundColor: isDark ? 'rgba(0,0,0,0.7)' : 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 300 }} onClick={onClose}>
    <div style={{ backgroundColor: colors.cardBg, borderRadius: '8px', padding: '20px', width: '320px', maxWidth: '90%' }} onClick={e => e.stopPropagation()}>
      <div style={{ fontSize: '14px', color: colors.text, marginBottom: '20px', lineHeight: '1.5', textAlign: 'center' }}>
        {message}
      </div>
      <div style={{ display: 'flex', justifyContent: 'center' }}>
        <button
          onClick={onClose}
          style={{ padding: '8px 24px', borderRadius: '6px', border: 'none', backgroundColor: '#3b82f6', color: '#fff', cursor: 'pointer' }}
        >
          OK
        </button>
      </div>
    </div>
  </div>
);
