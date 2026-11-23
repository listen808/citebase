import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import pluginVue from 'eslint-plugin-vue'
import prettierConfig from 'eslint-config-prettier'
import globals from 'globals'

export default tseslint.config(
  {
    ignores: [
      'dist/**',
      'node_modules/**',
      '*.d.ts',
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...pluginVue.configs['flat/recommended'],
  prettierConfig,
  {
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node,
      },
    },
  },
  {
    files: ['**/*.vue'],
    languageOptions: {
      parserOptions: {
        parser: tseslint.parser,
      },
    },
  },
  {
    rules: {
      'vue/component-name-in-template-casing': ['error', 'PascalCase'],
      'vue/multi-word-component-names': 'off',
      'vue/no-v-html': 'warn',
      'vue/require-default-prop': 'off',
      'vue/define-macros-order': ['error', {
        order: ['defineOptions', 'defineProps', 'defineEmits', 'defineSlots'],
      }],
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': ['warn', {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_',
      }],
      'no-console': ['warn', { allow: ['warn', 'error'] }],
      'prefer-const': 'error',
      'no-var': 'error',
      // 禁止使用 deprecated token 别名（karpathy: 简单规则，仅检测明确废弃的别名）
      'no-restricted-syntax': ['warn',
        // AppleType deprecated 别名
        // `caption` 原先列在这里，但那是清理前的旧别名。清理之后 **AppleType.caption 就是
        // 13px/400 这一档的唯一正名**（与 microUIBold 13px/600 配成一组），
        // `--apple-font-size-caption` 也直接从它派生。选择器分不出"旧废弃别名 caption"和
        // "现在这档 caption"，留着这条等于让人把正在用的 13px 改成 12px —— 它拦的不是别名，
        // 而是本档本身，故删。（其余各条对应的键确已从 AppleType 移除。）
        {
          selector: "MemberExpression[object.name='AppleType'][property.name='sectionHeading']",
          message: "AppleType.sectionHeading 已废弃，请使用 AppleType.cardProductTitle",
        },
        {
          selector: "MemberExpression[object.name='AppleType'][property.name='cardTitle']",
          message: "AppleType.cardTitle 已废弃，请使用 AppleType.bodyEmphasis",
        },
        {
          selector: "MemberExpression[object.name='AppleType'][property.name='subHeading']",
          message: "AppleType.subHeading 已废弃，请使用 AppleType.linkActionHeading 或 subhead",
        },
        {
          selector: "MemberExpression[object.name='AppleType'][property.name='body']",
          message: "AppleType.body 已废弃，请使用 AppleType.bodyPrimary",
        },
        {
          selector: "MemberExpression[object.name='AppleType'][property.name='button']",
          message: "AppleType.button 已废弃，请使用 AppleType.bodyEmphasis",
        },
        {
          selector: "MemberExpression[object.name='AppleType'][property.name='link']",
          message: "AppleType.link 已废弃，请使用 AppleType.bodyPrimary",
        },
        {
          selector: "MemberExpression[object.name='AppleType'][property.name='buttonSmall']",
          message: "AppleType.buttonSmall 已废弃，请使用 AppleType.microUI 并覆盖 weight",
        },
        // AppleRadius deprecated 别名
        {
          selector: "MemberExpression[object.name='AppleRadius'][property.name='standard']",
          message: "AppleRadius.standard 已废弃，请使用 AppleRadius.small",
        },
        {
          selector: "MemberExpression[object.name='AppleRadius'][property.name='comfortable']",
          message: "AppleRadius.comfortable 已废弃，请使用 AppleRadius.medium",
        },
        {
          selector: "MemberExpression[object.name='AppleRadius'][property.name='relaxed']",
          message: "AppleRadius.relaxed 已废弃，请使用 AppleRadius.medium",
        },
      ],
    },
  },
  {
    // web/scripts 下是门禁与生成脚本：stdout 就是它们的输出通道（报告、进度、自检结果），
    // 用 console 属于正常用法。src 与组件里仍然禁 console，走 logger。
    files: ['scripts/**/*.{mjs,js}'],
    rules: {
      'no-console': 'off',
    },
  },
  {
    // 测试里用 defineComponent 就地搭最小宿主组件是探针的常规写法（要复现"装了会不会触发"），
    // 不是"一个文件产出多个组件"，vue/one-component-per-file 在这类文件上不成立。
    files: ['**/*.test.ts', '**/*.spec.ts'],
    rules: {
      'vue/one-component-per-file': 'off',
    },
  },
)
