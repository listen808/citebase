import { appleFixedColumns, AppleFont, AppleRadius, AppleSpace, AppleTransition, AppleType, cv } from '@/utils/core/apple-tokens';

export const clearBtnStyle = {
  position: 'absolute' as const,
  right: '8px',
  top: '50%',
  transform: 'translateY(-50%)',
  background: 'none',
  border: 'none',
  color: cv('text-quaternary'),
  cursor: 'pointer',
  fontSize: AppleType.bodyPrimary.size,
  lineHeight: '1',
  padding: `0 ${AppleSpace.xs}`,
  borderRadius: AppleRadius.micro,
  transition: AppleTransition.fast,
};

export function labelStyle(error?: string) {
  return {
    fontFamily: AppleType.bodyEmphasis.font,
    fontSize: AppleType.controlLabel.size,
    fontWeight: String(AppleType.bodyEmphasis.weight),
    lineHeight: String(AppleType.bodyEmphasis.lineHeight),
    letterSpacing: AppleType.bodyEmphasis.letterSpacing,
    color: error ? cv('red') : cv('text-primary'),
  };
}

export const requiredStyle = {
  color: cv('red'),
  marginRight: AppleSpace.xs,
};

export function errorStyle() {
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.microUI.size,
    color: cv('red'),
    marginTop: AppleSpace.xs,
  };
}

export const wrapperStyle = {
  display: 'flex' as const,
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
  width: '100%',
};

// ============================================================
// Shared form layout styles (used across all views)
// ============================================================

export const formGroupStyle = {
  marginBottom: AppleSpace.md,
};

export const formGridStyle = {
  display: 'grid',
  gridTemplateColumns: appleFixedColumns(2),
  gap: AppleSpace.md,
  marginBottom: AppleSpace.md,
};
