import type { SubMenuItem, MenuGroup, ModuleMenuConfig } from '@code/common'
export type { SubMenuItem, MenuGroup, ModuleMenuConfig }

export const gateMenuConfig: ModuleMenuConfig = {
  moduleKey: 'gate',
  moduleName: 'AI 网关 (Code Gate)',
  groups: [
    {
      title: '网关体验',
      items: [
        {
          path: '/chat',
          label: '极速对话',
          icon: 'M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z',
        },
        {
          path: '/keys',
          label: '算力与密钥',
          icon: 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z',
        },
      ],
    },
    {
      title: '管控运维',
      adminOnly: true,
      items: [
        {
          path: '/admin/dashboard',
          label: '监控大屏',
          icon: 'M3 3v18h18M9 15l3-3 4 4 5-8',
        },
        {
          path: '/admin/users',
          label: '用户配额台账',
          icon: 'M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z',
        },
        {
          path: '/admin/backends',
          label: '模型与后端',
          icon: 'M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2',
        },
        {
          path: '/logs',
          label: '全链路审计日志',
          icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z',
        },
      ],
    },
  ],
}

export const menuGroups: MenuGroup[] = gateMenuConfig.groups
export const menuItems: SubMenuItem[] = gateMenuConfig.groups.flatMap((group) => group.items)

export default gateMenuConfig
