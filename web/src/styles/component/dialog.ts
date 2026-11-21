import { cv, AppleType, AppleSpace } from '@/utils/core/apple-tokens';

export function dialogHeaderStyle(hPadding: string = AppleSpace.lg) {
  return {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: `${AppleSpace.lg} ${hPadding}`,
    flexShrink: '0' as const,
    borderBottom: `1px solid ${cv('border-light')}`,
  };
}

export function dialogTitleStyle() {
  return {
    fontFamily: AppleType.bodyEmphasis.font,
    fontSize: AppleType.subhead.size,
    fontWeight: String(AppleType.bodyEmphasis.weight),
    margin: '0',
    color: cv('text-primary'),
    letterSpacing: AppleType.subhead.letterSpacing,
  };
}

export function dialogFooterStyle(hPadding: string = AppleSpace.lg) {
  return {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-end',
    gap: AppleSpace.sm,
    padding: `${AppleSpace.md} ${hPadding}`,
    flexShrink: '0' as const,
    borderTop: `1px solid ${cv('border-light')}`,
  };
}
