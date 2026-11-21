import { AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType, cv, appleCardGrid } from '@/utils/core/apple-tokens';
export const schemaContentStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.md };

export const schemaSpaceCardStyle = { padding: AppleSpace.sm, borderBottom: `1px solid ${cv('border-light')}` };
export const schemaSpaceHeaderStyle = { display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: AppleSpace.xs };
export const schemaSpaceInfoStyle = { flex: 1 };
export const schemaSpaceNameStyle = { fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary'), marginBottom: AppleSpace.xs };
export const schemaSpaceMetaStyle = { display: 'flex', flexWrap: 'wrap' as const, gap: AppleSpace.sm, alignItems: 'center' };
export const schemaSpaceMetaItemStyle = { display: 'inline-flex', alignItems: 'center', gap: AppleSpace.xxs, fontSize: AppleType.legalMeta.size, color: cv('text-tertiary') };
export const schemaSpaceScanInfoStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.xxs, fontSize: AppleType.legalMeta.size, color: cv('text-tertiary') };
export const schemaSpaceActionsStyle = { display: 'flex', gap: AppleSpace.xs };

export const schemaDbTypeBadgeStyle = (dbType: string) => ({
  fontFamily: 'monospace', fontSize: AppleType.legalMeta.size, padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.micro,
  backgroundColor: dbType === 'mysql' ? `${cv('blue-bg-subtle')}` : dbType === 'postgresql' ? `${cv('purple-bg-subtle')}` : `${cv('orange-bg-subtle')}`,
  color: dbType === 'mysql' ? cv('blue') : dbType === 'postgresql' ? cv('purple') : cv('orange'),
});

export const schemaFilterRowStyle = { display: 'flex', gap: AppleSpace.sm, alignItems: 'center', marginBottom: AppleSpace.md };

export const schemaTableListStyle = { ...appleCardGrid('entity', AppleSpace.xs), marginBottom: AppleSpace.md };
export const schemaTableItemStyle = (active: boolean) => ({
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`, borderRadius: AppleRadius.small, cursor: 'pointer',
  background: active ? `${cv('blue-bg-subtle')}` : cv('bg-secondary'),
  border: `1px solid ${active ? cv('blue') : cv('border-light')}`,
});
export const schemaTableItemNameStyle = { fontFamily: 'monospace', fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.medium), color: cv('text-primary') };
export const schemaTableItemCommentStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-tertiary'), marginTop: '2px' };

export const schemaTableDetailStyle = { background: cv('surface-panel'), borderRadius: AppleRadius.card, border: `1px solid ${cv('border-light')}`, padding: AppleSpace.md };
export const schemaDetailHeaderStyle = { marginBottom: AppleSpace.sm };
export const schemaDetailTitleStyle = { fontFamily: 'monospace', fontSize: AppleType.subhead.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary') };
export const schemaDetailCommentStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-tertiary'), marginTop: AppleSpace.xs };

export const schemaTypeBadgeStyle = { fontFamily: 'monospace', fontSize: AppleType.legalMeta.size, padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.micro, background: cv('bg-secondary'), color: cv('text-secondary') };
export const schemaPkBadgeStyle = { padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, background: `${cv('green-bg-subtle')}`, color: cv('green'), fontSize: AppleType.legalMeta.size, fontWeight: String(AppleFontWeight.semibold) };
export const schemaFkBadgeStyle = { padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, background: `${cv('blue-bg-subtle')}`, color: cv('blue'), fontSize: AppleType.legalMeta.size, fontWeight: String(AppleFontWeight.semibold) };
export const schemaIdxBadgeStyle = { padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, background: `${cv('orange-bg-subtle')}`, color: cv('orange'), fontSize: AppleType.legalMeta.size, fontWeight: String(AppleFontWeight.semibold) };
export const schemaTableCountStyle = { display: 'flex', alignItems: 'center', marginBottom: AppleSpace.xs, paddingLeft: AppleSpace.xs };
export const schemaTableCountTextStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), fontWeight: String(AppleFontWeight.medium) };
export const schemaTableCountSubStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-tertiary') };

export const schemaRelationsSectionStyle = { marginTop: AppleSpace.sm };
export const schemaRelTitleStyle = { fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary'), marginBottom: AppleSpace.xs };
export const schemaRelItemStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.xs, padding: `${AppleSpace.xxs} 0`, fontSize: AppleType.legalMeta.size, color: cv('text-secondary') };
export const schemaRelTypeBadgeStyle = { padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, background: `${cv('blue-bg-subtle')}`, color: cv('blue'), fontSize: AppleType.legalMeta.size };

export const schemaRuleCardStyle = { padding: AppleSpace.sm, borderBottom: `1px solid ${cv('border-light')}` };
export const schemaRuleHeaderStyle = { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: AppleSpace.xxs };
export const schemaRuleNameStyle = { fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary') };
export const schemaRuleBadgesStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.sm };
export const schemaRuleTypeBadgeStyle = { padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, background: `${cv('purple-bg-subtle')}`, color: cv('purple'), fontSize: AppleType.legalMeta.size };
export const schemaRuleTargetStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.xxs, fontSize: AppleType.legalMeta.size, color: cv('text-tertiary'), marginBottom: AppleSpace.xxs };
export const schemaRuleContentStyle = { fontSize: AppleType.bodyPrimary.size, color: cv('text-primary'), lineHeight: 1.5, marginBottom: AppleSpace.xxs };
export const schemaRuleDescStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), lineHeight: 1.4 };

export const schemaAskInputAreaStyle = { background: cv('surface-panel'), borderRadius: AppleRadius.card, border: `1px solid ${cv('border-light')}`, padding: AppleSpace.md, marginBottom: AppleSpace.md };
export const schemaAskTextareaStyle = { width: '100%', minHeight: '80px', padding: AppleSpace.sm, border: `1px solid ${cv('border-light')}`, borderRadius: AppleRadius.small, background: cv('bg-secondary'), color: cv('text-primary'), fontSize: AppleType.bodyPrimary.size, fontFamily: AppleFont.text, resize: 'vertical' as const, lineHeight: 1.5 };
export const schemaAskActionsStyle = { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: AppleSpace.xs };
export const schemaAskHintStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-tertiary') };

export const schemaAskResultStyle = { background: cv('bg-secondary'), borderRadius: AppleRadius.card, border: `1px solid ${cv('border-light')}`, padding: AppleSpace.md };
export const schemaResultSectionStyle = { marginBottom: AppleSpace.sm };
export const schemaResultLabelStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.xs, fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary'), marginBottom: AppleSpace.xs };
export const schemaSqlBlockStyle = { background: cv('bg-secondary'), color: cv('text-primary'), padding: AppleSpace.sm, borderRadius: AppleRadius.small, fontFamily: AppleFont.mono, fontSize: AppleType.codeBlock.size, lineHeight: String(AppleType.codeBlock.lineHeight), margin: 0, overflow: 'auto' as const, maxHeight: '400px', border: `1px solid ${cv('border-light')}` };
export const schemaExplanationStyle = { fontSize: AppleType.bodyPrimary.size, color: cv('text-secondary'), lineHeight: 1.6 };

export const schemaConfidenceBarStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.sm, marginBottom: AppleSpace.sm };
export const schemaConfLabelStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), flexShrink: 0 };
export const schemaConfValueStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), fontWeight: String(AppleFontWeight.semibold), flexShrink: 0 };

export const schemaSourcesStyle = { marginBottom: AppleSpace.sm };
export const schemaSourceListStyle = { display: 'flex', flexWrap: 'wrap' as const, gap: AppleSpace.xs, marginTop: AppleSpace.xs };
export const schemaSourceTagStyle = { padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, background: `${cv('blue-bg-subtle')}`, color: cv('blue'), fontSize: AppleType.legalMeta.size, display: 'inline-flex', alignItems: 'center', gap: AppleSpace.xxs };
export const schemaSourceConfStyle = { color: cv('text-tertiary'), marginLeft: AppleSpace.xxs };

export const schemaWarningsStyle = { marginTop: AppleSpace.sm };
export const schemaWarningItemStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.xs, padding: `${AppleSpace.xxs} 0`, fontSize: AppleType.legalMeta.size, color: cv('orange') };
