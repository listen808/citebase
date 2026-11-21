/**
 * 为非原生交互元素（如 `<div @click>`）提供键盘可访问性。
 *
 * WCAG 2.1.1 要求所有功能可通过键盘操作。可点击的 div/span 缺少
 * role/tabindex/键盘事件，键盘用户无法到达。本工具返回可展开的
 * props，配合 `v-bind` 一次性补齐 role="button"、tabindex、click
 * 与 Enter/Space 键盘激活。
 *
 * @example
 * <!-- 修复前 -->
 * <div @click="select(item)" :style="cardStyle">...</div>
 * <!-- 修复后 -->
 * <div v-bind="keyboardClick(() => select(item))" :style="cardStyle">...</div>
 * <!-- 菜单项需要 role="menuitem"（role="button" 不是 menu 的合法子角色） -->
 * <div v-bind="keyboardClick(open, 'menuitem')">...</div>
 */
export function keyboardClick(
  handler: () => void,
  role: 'button' | 'menuitem' = 'button',
): {
  role: 'button' | 'menuitem';
  tabindex: number;
  onClick: () => void;
  onKeydown: (e: KeyboardEvent) => void;
} {
  return {
    role,
    tabindex: 0,
    onClick: handler,
    onKeydown(e: KeyboardEvent) {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        handler();
      }
    },
  };
}