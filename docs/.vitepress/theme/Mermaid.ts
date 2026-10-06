import { defineComponent, h, onMounted, ref, useId, watch } from 'vue';
import { useData } from 'vitepress';

export default defineComponent({
	props: { code: { type: String, required: true } },
	setup(props) {
		const { isDark } = useData();
		const id = `mermaid-${useId()}`;
		const svg = ref('');

		async function render() {
			const { default: mermaid } = await import('mermaid');
			await document.fonts.ready;
			const css = getComputedStyle(document.documentElement);
			const token = (name: string) => css.getPropertyValue(name).trim();
			const fontFamily = token('--vp-font-family-base');
			mermaid.initialize({
				startOnLoad: false,
				theme: 'base',
				fontFamily,
				themeVariables: {
					darkMode: isDark.value,
					fontFamily,
					fontSize: '14px',
					background: token('--vp-c-bg-alt'),
					primaryColor: token('--vp-c-bg'),
					primaryTextColor: token('--vp-c-text-1'),
					primaryBorderColor: token('--vp-c-brand-1'),
					secondaryColor: token('--vp-c-bg-soft'),
					tertiaryColor: token('--vp-c-bg-soft'),
					lineColor: token('--vp-c-text-2'),
					textColor: token('--vp-c-text-1'),
					clusterBkg: token('--vp-c-bg-soft'),
					clusterBorder: token('--vp-c-divider'),
					edgeLabelBackground: token('--vp-c-bg-alt'),
					noteBkgColor: token('--vp-c-bg-soft'),
					noteBorderColor: token('--vp-c-divider'),
					noteTextColor: token('--vp-c-text-1')
				},
				flowchart: { useMaxWidth: false, padding: 12 },
				sequence: { useMaxWidth: false },
				state: { useMaxWidth: false },
				timeline: { useMaxWidth: false }
			});
			const result = await mermaid.render(id, decodeURIComponent(props.code));
			svg.value = result.svg;
		}

		onMounted(render);
		watch(isDark, render, { flush: 'post' });
		return () => h('div', { class: 'mermaid', innerHTML: svg.value });
	}
});
