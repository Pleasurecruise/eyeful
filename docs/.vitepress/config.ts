import { writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import {
	transformerMetaWordHighlight,
	transformerNotationWordHighlight
} from '@shikijs/transformers';
import { defineConfig, type DefaultTheme } from 'vitepress';
import llmstxt from 'vitepress-plugin-llms';

const docsUrl = process.env.DOCS_URL;
if (!docsUrl) throw new Error('DOCS_URL is not set; run the docs through mise');
const site = new URL(docsUrl);
const base = site.pathname.endsWith('/') ? site.pathname : `${site.pathname}/`;
const repo = 'https://github.com/Pleasurecruise/eyeful';

const groups = [
	[
		'overview',
		'PRINCIPLES',
		'REVIEW',
		'PLANNER',
		'LEVELS',
		'EXPERTS',
		'VERIFICATION',
		'REPORT',
		'LOCAL',
		'CLOUD',
		'FAQ',
		'REFERENCES'
	],
	['ARCHITECTURE', 'SCHEDULING', 'AUTH', 'PERSISTENCE', 'API', 'DEVELOPMENT', 'STYLEGUIDE'],
	['DEPLOYMENT', 'CONFIGURATION'],
	['ROADMAP'],
	['EVALUATION']
];

function sidebar(prefix: string, titles: string[], labels: string[][]): DefaultTheme.Sidebar {
	return groups.map((files, i) => ({
		text: titles[i],
		items: files.map((file, j) => ({ text: labels[i][j], link: `${prefix}/${file}` }))
	}));
}

function pagePath(relativePath: string): string {
	return relativePath
		.replace(/(^|\/)index\.md$/, '$1')
		.replace(/(^|\/)README\.md$/, '$1overview')
		.replace(/\.md$/, '');
}

export default defineConfig({
	title: 'eyeful',
	base,
	cleanUrls: true,
	lastUpdated: true,
	rewrites: { 'README.md': 'overview.md', 'zh/README.md': 'zh/overview.md' },
	ignoreDeadLinks: [/^\.\.\//],
	markdown: {
		theme: { light: 'github-light', dark: 'github-dark' },
		codeTransformers: [transformerNotationWordHighlight(), transformerMetaWordHighlight()],
		config(md) {
			const fence = md.renderer.rules.fence;
			if (!fence) throw new Error('markdown-it has no fence renderer');
			md.renderer.rules.fence = (tokens, idx, options, env, self) =>
				tokens[idx].info.trim() === 'mermaid'
					? `<Mermaid code="${encodeURIComponent(tokens[idx].content)}" />`
					: fence(tokens, idx, options, env, self);
		}
	},
	sitemap: { hostname: site.href },
	head: [
		['link', { rel: 'icon', type: 'image/svg+xml', href: `${base}favicon.svg` }],
		['meta', { name: 'theme-color', content: '#3451b2' }],
		['meta', { property: 'og:type', content: 'website' }],
		['meta', { property: 'og:site_name', content: 'eyeful' }]
	],
	locales: {
		root: {
			label: 'English',
			lang: 'en',
			description: 'Multi-agent code review with executable evidence',
			themeConfig: {
				sidebar: sidebar(
					'',
					['How eyeful reviews', 'How the code is built', 'How it is deployed', 'Plan', 'Thesis'],
					[
						[
							'Overview',
							'Principles',
							'A review, end to end',
							'Planning and triage',
							'Levels and lazy activation',
							'Expert orchestration',
							'Evidence and verification',
							'Review feedback',
							'Running locally',
							'Running in the cloud',
							'Questions and answers',
							'References'
						],
						[
							'Architecture',
							'Scheduling and leases',
							'Sign-in and credentials',
							'Database and migrations',
							'API',
							'Local development',
							'Code style and review'
						],
						['Deployment', 'Configuration reference'],
						['Roadmap'],
						['Evaluation']
					]
				)
			}
		},
		zh: {
			label: '简体中文',
			lang: 'zh-CN',
			link: '/zh/',
			description: '以可执行证据为基础的多智能体代码审查',
			themeConfig: {
				sidebar: sidebar(
					'/zh',
					['项目思路', '项目框架', '项目部署', '计划', '毕业设计'],
					[
						[
							'eyeful 是什么',
							'八条原则',
							'一次审查的全过程',
							'规划与分诊',
							'档位与懒激活',
							'专家编排',
							'证据与验证',
							'审查反馈',
							'本地运行',
							'云端运行',
							'常见问题',
							'参考文献'
						],
						[
							'整体架构',
							'任务调度与租约',
							'登录与凭据',
							'数据库与迁移',
							'API 约定',
							'本地开发',
							'代码规范与评审'
						],
						['部署上线', '配置项参考'],
						['版本路线图'],
						['效果评测']
					]
				),
				outline: { label: '本页目录' },
				docFooter: { prev: '上一页', next: '下一页' },
				lastUpdated: { text: '最后更新' },
				returnToTopLabel: '回到顶部',
				sidebarMenuLabel: '菜单',
				darkModeSwitchLabel: '外观',
				lightModeSwitchTitle: '切换到浅色模式',
				darkModeSwitchTitle: '切换到深色模式',
				langMenuLabel: '切换语言',
				notFound: { title: '页面不存在', linkText: '返回首页', quote: '这个地址没有对应的页面。' }
			}
		}
	},
	transformHead({ pageData, title, description }) {
		const path = pagePath(pageData.relativePath);
		const english = path.replace(/^zh\/?/, '');
		const url = new URL(path, site).href;
		return [
			['meta', { property: 'og:title', content: title }],
			['meta', { property: 'og:description', content: description }],
			['meta', { property: 'og:url', content: url }],
			['link', { rel: 'canonical', href: url }],
			['link', { rel: 'alternate', hreflang: 'en', href: new URL(english, site).href }],
			['link', { rel: 'alternate', hreflang: 'zh-CN', href: new URL(`zh/${english}`, site).href }],
			['link', { rel: 'alternate', hreflang: 'x-default', href: new URL(english, site).href }]
		];
	},
	async buildEnd({ outDir }) {
		await writeFile(
			join(outDir, 'robots.txt'),
			`User-agent: *\nAllow: /\n\nSitemap: ${new URL('sitemap.xml', site).href}\n`
		);
	},
	vite: { plugins: [llmstxt({ domain: site.origin, ignoreFiles: ['zh/**'] })] },
	themeConfig: {
		logo: '/favicon.svg',
		search: {
			provider: 'local',
			options: {
				locales: {
					zh: {
						translations: {
							button: { buttonText: '搜索', buttonAriaLabel: '搜索' },
							modal: {
								noResultsText: '没有找到相关结果',
								resetButtonTitle: '清除',
								footer: { selectText: '选择', navigateText: '切换', closeText: '关闭' }
							}
						}
					}
				}
			}
		},
		socialLinks: [{ icon: 'github', link: repo }]
	}
});
