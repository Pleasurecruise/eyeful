import { getLocale, overwriteGetLocale, setLocale, type Locale } from '#i18n/runtime';

const initial = getLocale();
let current = $state(initial);

overwriteGetLocale(() => current);
document.documentElement.lang = initial;

export function switchLocale(locale: Locale) {
	void setLocale(locale, { reload: false });
	current = locale;
	document.documentElement.lang = locale;
}
