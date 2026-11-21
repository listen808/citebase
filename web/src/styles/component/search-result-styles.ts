import { AppleRadius, AppleSpace, AppleTransition, cv } from '@/utils/core/apple-tokens';

// ==================== Search Result 样式 ====================

export function searchResultCardStyle(): Record<string, string> {
  return {
    padding: `${AppleSpace.md} ${AppleSpace.md}`,
    cursor: 'pointer',
    display: 'flex',
    flexDirection: 'column',
    gap: AppleSpace.xs,
    background: cv('surface-panel'),
    borderRadius: AppleRadius.card,
    border: `1px solid ${cv('border-light')}`,
    boxShadow: cv('shadow-card'),
    transition: AppleTransition.smooth,
  };
}
