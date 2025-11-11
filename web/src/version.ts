// 由 Vite 注入，若未定义则使用 fallback
export const APP_VERSION = (typeof __APP_VERSION__ !== 'undefined') ? __APP_VERSION__ : 'f8713'