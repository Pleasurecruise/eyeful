<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { Button, DropdownMenu, setMode, userPrefersMode } from '@eyeful/ui';
	import { Languages, LogOut, Monitor, Moon, Sun } from '@lucide/svelte';
	import { m } from '#i18n';
	import { switchLocale } from '#lib/locale.svelte.ts';
	import { getLocale, locales } from '#i18n/runtime';
	import { session } from '#lib/session.svelte.ts';

	let { children } = $props();

	$effect(() => {
		if (!session.user) goto(resolve('/login'), { replaceState: true });
	});

	async function signOut() {
		await session.signOut();
		await goto(resolve('/login'));
	}
</script>

<div class="min-h-svh bg-background text-foreground">
	<header class="border-b">
		<div class="mx-auto flex h-14 max-w-5xl items-center gap-6 px-4">
			<a href={resolve('/(console)/repositories')} class="font-semibold tracking-tight">eyeful</a>
			<div class="ml-auto flex items-center gap-2">
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Button {...props} variant="ghost" size="icon" aria-label={m.language()}>
								<Languages />
							</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end">
						<DropdownMenu.RadioGroup value={getLocale()}>
							{#each locales as locale (locale)}
								<DropdownMenu.RadioItem value={locale} onclick={() => switchLocale(locale)}>
									{new Intl.DisplayNames([locale], { type: 'language' }).of(locale)}
								</DropdownMenu.RadioItem>
							{/each}
						</DropdownMenu.RadioGroup>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Button {...props} variant="ghost" size="icon" aria-label={m.theme()}>
								{#if userPrefersMode.current === 'dark'}
									<Moon />
								{:else if userPrefersMode.current === 'light'}
									<Sun />
								{:else}
									<Monitor />
								{/if}
							</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end">
						<DropdownMenu.Item onclick={() => setMode('light')}
							><Sun /> {m.theme_light()}</DropdownMenu.Item
						>
						<DropdownMenu.Item onclick={() => setMode('dark')}
							><Moon /> {m.theme_dark()}</DropdownMenu.Item
						>
						<DropdownMenu.Item onclick={() => setMode('system')}
							><Monitor /> {m.theme_system()}</DropdownMenu.Item
						>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
				{#if session.user}
					{@const user = session.user}
					<DropdownMenu.Root>
						<DropdownMenu.Trigger>
							{#snippet child({ props })}
								<Button {...props} variant="outline" size="sm">{user.email}</Button>
							{/snippet}
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="end">
							<DropdownMenu.Item onclick={signOut}><LogOut /> {m.sign_out()}</DropdownMenu.Item>
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				{/if}
			</div>
		</div>
	</header>
	<main class="mx-auto max-w-5xl px-4 py-8">
		{#if session.user}
			{@render children()}
		{/if}
	</main>
</div>
