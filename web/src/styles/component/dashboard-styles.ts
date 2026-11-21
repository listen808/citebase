import { AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleTransition, AppleType, cv } from '@/utils/core/apple-tokens';





export function monitorGridStyle(): Record<string, string> {
  return {
    display: 'grid',
    gridTemplateColumns: '1fr',
    gap: AppleSpace.md,
  };
}

export function monitorLabelStyle(): Record<string, string> {
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.controlLabel.size,
    color: cv('text-secondary'),
  };
}

export function monitorValueStyle(): Record<string, string> {
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.subhead.size,
    fontWeight: String(AppleFontWeight.semibold),
    color: cv('text-primary'),
  };
}

export function rankListStyle(): Record<string, string> {
  return {
    display: 'flex',
    flexDirection: 'column',
    gap: AppleSpace.sm,
  };
}

export function rankItemStyle(_rank: number): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.md,
    padding: `${AppleSpace.sm} ${AppleSpace.md}`,
    borderRadius: AppleRadius.control,
    transition: AppleTransition.smooth,
  };
}

export function rankNumberStyle(rank: number): Record<string, string> {
  return {
    width: AppleSpace.lg,
    flexShrink: '0',
    fontFamily: AppleFont.text,
    fontSize: AppleType.microUI.size,
    fontWeight: String(AppleFontWeight.semibold),
    fontVariantNumeric: 'tabular-nums',
    textAlign: 'right' as const,
    color: rank <= 3 ? cv('text-primary') : cv('text-tertiary'),
  };
}

export function rankNameStyle(): Record<string, string> {
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.controlLabel.size,
    color: cv('text-primary'),
    flex: '1',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  };
}

export function rankCountStyle(): Record<string, string> {
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.microUI.size,
    color: cv('text-tertiary'),
    flexShrink: '0',
  };
}
