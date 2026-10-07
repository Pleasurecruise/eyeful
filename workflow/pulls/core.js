//#region \0rolldown/runtime.js
var __create = Object.create;
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __getProtoOf = Object.getPrototypeOf;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __commonJSMin = (cb, mod) => () => (mod || (cb((mod = { exports: {} }).exports, mod), cb = null), mod.exports);
var __copyProps = (to, from, except, desc) => {
	if (from && typeof from === "object" || typeof from === "function") for (var keys = __getOwnPropNames(from), i = 0, n = keys.length, key; i < n; i++) {
		key = keys[i];
		if (!__hasOwnProp.call(to, key) && key !== except) __defProp(to, key, {
			get: ((k) => from[k]).bind(null, key),
			enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable
		});
	}
	return to;
};
var __toESM = (mod, isNodeMode, target) => (target = mod != null ? __create(__getProtoOf(mod)) : {}, __copyProps(isNodeMode || !mod || !mod.__esModule || !__hasOwnProp.call(mod, "default") ? __defProp(target, "default", {
	value: mod,
	enumerable: true
}) : target, mod));
var DEFAULT_CONFIG = {
	lang: void 0,
	message: void 0,
	abortEarly: void 0,
	abortPipeEarly: void 0
};
/**
* Returns the global configuration.
*
* @param config The config to merge.
*
* @returns The configuration.
*/
/* @__NO_SIDE_EFFECTS__ */
function getGlobalConfig(config$1) {
	if (!config$1 && true) return DEFAULT_CONFIG;
	return {
		lang: config$1?.lang ?? void 0,
		message: config$1?.message,
		abortEarly: config$1?.abortEarly ?? void 0,
		abortPipeEarly: config$1?.abortPipeEarly ?? void 0
	};
}
/**
* Stringifies an unknown input to a literal or type string.
*
* @param input The unknown input.
*
* @returns A literal or type string.
*
* @internal
*/
/* @__NO_SIDE_EFFECTS__ */
function _stringify(input) {
	const type = typeof input;
	if (type === "string") return `"${input}"`;
	if (type === "number" || type === "bigint" || type === "boolean") return `${input}`;
	if (type === "object" || type === "function") return (input && Object.getPrototypeOf(input)?.constructor?.name) ?? "null";
	return type;
}
/**
* Adds an issue to the dataset.
*
* @param context The issue context.
* @param label The issue label.
* @param dataset The input dataset.
* @param config The configuration.
* @param other The optional props.
*
* @internal
*/
function _addIssue(context, label, dataset, config$1, other) {
	const input = other && "input" in other ? other.input : dataset.value;
	const expected = other?.expected ?? context.expects ?? null;
	const received = other?.received ?? /* @__PURE__ */ _stringify(input);
	const issue = {
		kind: context.kind,
		type: context.type,
		input,
		expected,
		received,
		message: `Invalid ${label}: ${expected ? `Expected ${expected} but r` : "R"}eceived ${received}`,
		requirement: context.requirement,
		path: other?.path,
		issues: other?.issues,
		lang: config$1.lang,
		abortEarly: config$1.abortEarly,
		abortPipeEarly: config$1.abortPipeEarly
	};
	const isSchema = context.kind === "schema";
	const message$1 = other?.message ?? context.message ?? (context.reference, issue.lang, void 0) ?? (isSchema ? (issue.lang, void 0) : null) ?? config$1.message ?? (issue.lang, void 0);
	if (message$1 !== void 0) issue.message = typeof message$1 === "function" ? message$1(issue) : message$1;
	if (isSchema) dataset.typed = false;
	if (dataset.issues) dataset.issues.push(issue);
	else dataset.issues = [issue];
}
/**
* Joins multiple `expects` values with the given separator.
*
* @param values The `expects` values.
* @param separator The separator.
*
* @returns The joined `expects` property.
*
* @internal
*/
/* @__NO_SIDE_EFFECTS__ */
function _joinExpects(values$1, separator) {
	const list = [...new Set(values$1)];
	if (list.length > 1) return `(${list.join(` ${separator} `)})`;
	return list[0] ?? "never";
}
/**
* Eagerly creates and attaches the Standard Schema properties of a schema.
*
* Hint: The contextual `this` type includes the standard properties that are
* attached before the schema is returned.
*
* @param schema The schema to attach standard properties to.
*
* @returns The schema with standard properties attached.
*
* @internal
*/
function _standardSchema(schema) {
	schema["~standard"] = {
		version: 1,
		vendor: "valibot",
		validate: (value$1) => schema["~run"]({ value: value$1 }, /* @__PURE__ */ getGlobalConfig())
	};
	return schema;
}
/* @__NO_SIDE_EFFECTS__ */
function getDotPath(issue) {
	if (issue.path) {
		let key = "";
		for (const item of issue.path) if (typeof item.key === "string" || typeof item.key === "number") if (key) key += `.${item.key}`;
		else key += item.key;
		else return null;
		return key;
	}
	return null;
}
/**
* Creates a description metadata action.
*
* @param description_ The description text.
*
* @returns A description action.
*/
/* @__NO_SIDE_EFFECTS__ */
function description(description_) {
	return {
		kind: "metadata",
		type: "description",
		reference: description,
		description: description_
	};
}
/**
* Returns the fallback value of the schema.
*
* @param schema The schema to get it from.
* @param dataset The output dataset if available.
* @param config The config if available.
*
* @returns The fallback value.
*/
/* @__NO_SIDE_EFFECTS__ */
function getFallback(schema, dataset, config$1) {
	return typeof schema.fallback === "function" ? schema.fallback(dataset, config$1) : schema.fallback;
}
/**
* Returns the default value of the schema.
*
* @param schema The schema to get it from.
* @param dataset The input dataset if available.
* @param config The config if available.
*
* @returns The default value.
*/
/* @__NO_SIDE_EFFECTS__ */
function getDefault(schema, dataset, config$1) {
	return typeof schema.default === "function" ? schema.default(dataset, config$1) : schema.default;
}
/* @__NO_SIDE_EFFECTS__ */
function array(item, message$1) {
	return _standardSchema({
		kind: "schema",
		type: "array",
		reference: array,
		expects: "Array",
		async: false,
		item,
		message: message$1,
		"~run"(dataset, config$1) {
			const input = dataset.value;
			if (Array.isArray(input)) {
				dataset.typed = true;
				dataset.value = [];
				for (let key = 0; key < input.length; key++) {
					const value$1 = input[key];
					const itemDataset = this.item["~run"]({ value: value$1 }, config$1);
					if (itemDataset.issues) {
						const pathItem = {
							type: "array",
							origin: "value",
							input,
							key,
							value: value$1
						};
						for (const issue of itemDataset.issues) {
							if (issue.path) issue.path.unshift(pathItem);
							else issue.path = [pathItem];
							dataset.issues?.push(issue);
						}
						if (!dataset.issues) dataset.issues = itemDataset.issues;
						if (config$1.abortEarly) {
							dataset.typed = false;
							break;
						}
					}
					if (!itemDataset.typed) dataset.typed = false;
					dataset.value.push(itemDataset.value);
				}
			} else _addIssue(this, "type", dataset, config$1);
			return dataset;
		}
	});
}
/* @__NO_SIDE_EFFECTS__ */
function boolean(message$1) {
	return _standardSchema({
		kind: "schema",
		type: "boolean",
		reference: boolean,
		expects: "boolean",
		async: false,
		message: message$1,
		"~run"(dataset, config$1) {
			if (typeof dataset.value === "boolean") dataset.typed = true;
			else _addIssue(this, "type", dataset, config$1);
			return dataset;
		}
	});
}
/* @__NO_SIDE_EFFECTS__ */
function number(message$1) {
	return _standardSchema({
		kind: "schema",
		type: "number",
		reference: number,
		expects: "number",
		async: false,
		message: message$1,
		"~run"(dataset, config$1) {
			if (typeof dataset.value === "number" && !isNaN(dataset.value)) dataset.typed = true;
			else _addIssue(this, "type", dataset, config$1);
			return dataset;
		}
	});
}
/* @__NO_SIDE_EFFECTS__ */
function object(entries$1, message$1) {
	return _standardSchema({
		kind: "schema",
		type: "object",
		reference: object,
		expects: "Object",
		async: false,
		entries: entries$1,
		message: message$1,
		"~run"(dataset, config$1) {
			const input = dataset.value;
			if (input && typeof input === "object") {
				dataset.typed = true;
				dataset.value = {};
				for (const key in this.entries) {
					const valueSchema = this.entries[key];
					if (key in input || (valueSchema.type === "exact_optional" || valueSchema.type === "optional" || valueSchema.type === "nullish") && valueSchema.default !== void 0) {
						const value$1 = key in input ? input[key] : /* @__PURE__ */ getDefault(valueSchema);
						const valueDataset = valueSchema["~run"]({ value: value$1 }, config$1);
						if (valueDataset.issues) {
							const pathItem = {
								type: "object",
								origin: "value",
								input,
								key,
								value: value$1
							};
							for (const issue of valueDataset.issues) {
								if (issue.path) issue.path.unshift(pathItem);
								else issue.path = [pathItem];
								dataset.issues?.push(issue);
							}
							if (!dataset.issues) dataset.issues = valueDataset.issues;
							if (config$1.abortEarly) {
								dataset.typed = false;
								break;
							}
						}
						if (!valueDataset.typed) dataset.typed = false;
						dataset.value[key] = valueDataset.value;
					} else if (valueSchema.fallback !== void 0) dataset.value[key] = /* @__PURE__ */ getFallback(valueSchema);
					else if (valueSchema.type !== "exact_optional" && valueSchema.type !== "optional" && valueSchema.type !== "nullish") {
						_addIssue(this, "key", dataset, config$1, {
							input: void 0,
							expected: `"${key}"`,
							path: [{
								type: "object",
								origin: "key",
								input,
								key,
								value: input[key]
							}]
						});
						if (config$1.abortEarly) break;
					}
				}
			} else _addIssue(this, "type", dataset, config$1);
			return dataset;
		}
	});
}
/* @__NO_SIDE_EFFECTS__ */
function optional(wrapped, default_) {
	return _standardSchema({
		kind: "schema",
		type: "optional",
		reference: optional,
		expects: `(${wrapped.expects} | undefined)`,
		async: false,
		wrapped,
		default: default_,
		"~run"(dataset, config$1) {
			if (dataset.value === void 0) {
				if (this.default !== void 0) dataset.value = /* @__PURE__ */ getDefault(this, dataset, config$1);
				if (dataset.value === void 0) {
					dataset.typed = true;
					return dataset;
				}
			}
			return this.wrapped["~run"](dataset, config$1);
		}
	});
}
/* @__NO_SIDE_EFFECTS__ */
function picklist(options, message$1) {
	return _standardSchema({
		kind: "schema",
		type: "picklist",
		reference: picklist,
		expects: /* @__PURE__ */ _joinExpects(options.map(_stringify), "|"),
		async: false,
		options,
		message: message$1,
		"~run"(dataset, config$1) {
			if (this.options.includes(dataset.value)) dataset.typed = true;
			else _addIssue(this, "type", dataset, config$1);
			return dataset;
		}
	});
}
/* @__NO_SIDE_EFFECTS__ */
function string(message$1) {
	return _standardSchema({
		kind: "schema",
		type: "string",
		reference: string,
		expects: "string",
		async: false,
		message: message$1,
		"~run"(dataset, config$1) {
			if (typeof dataset.value === "string") dataset.typed = true;
			else _addIssue(this, "type", dataset, config$1);
			return dataset;
		}
	});
}
/* @__NO_SIDE_EFFECTS__ */
function pipe(...pipe$1) {
	return _standardSchema({
		...pipe$1[0],
		pipe: pipe$1,
		"~run"(dataset, config$1) {
			for (const item of pipe$1) if (item.kind !== "metadata") {
				if (dataset.issues && (item.kind === "schema" || item.kind === "transformation")) {
					dataset.typed = false;
					break;
				}
				if (!dataset.issues || !config$1.abortEarly && !config$1.abortPipeEarly) dataset = item["~run"](dataset, config$1);
			}
			return dataset;
		}
	});
}
/**
* Parses an unknown input based on a schema.
*
* @param schema The schema to be used.
* @param input The input to be parsed.
* @param config The parse configuration.
*
* @returns The parse result.
*/
/* @__NO_SIDE_EFFECTS__ */
function safeParse(schema, input, config$1) {
	const dataset = schema["~run"]({ value: input }, /* @__PURE__ */ getGlobalConfig(config$1));
	return {
		typed: dataset.typed,
		success: !dataset.issues,
		output: dataset.value,
		issues: dataset.issues
	};
}
/**
* Summarize the error messages of issues in a pretty-printable multi-line string.
*
* @param issues The list of issues.
*
* @returns A summary of the issues.
*
* @beta
*/
/* @__NO_SIDE_EFFECTS__ */
function summarize(issues) {
	let summary = "";
	for (const issue of issues) {
		if (summary) summary += "\n";
		summary += `× ${issue.message}`;
		const dotPath = /* @__PURE__ */ getDotPath(issue);
		if (dotPath) summary += `\n  → at ${dotPath}`;
	}
	return summary;
}
//#endregion
//#region ../../node_modules/.pnpm/@pulls.review+core@0.3.1_typescript@6.0.3_ws@8.22.0_zod@4.6.5/node_modules/@pulls.review/core/dist/analyze-D-y_E6as.js
/**
* Review-comment threads and review summaries for a GitHub PR, in the same
* valibot-schema-first style as `diff.ts`. Sides use pierre's vocabulary
* (`additions`/`deletions`) rather than GitHub's `RIGHT`/`LEFT` so the view
* layer never translates; the github provider maps at the boundary.
*/
var DiffSideSchema = /* @__PURE__ */ picklist(["additions", "deletions"]);
/**
* Which part of the system a group touches - one axis, language-agnostic, so the view
* can give every group a recognizable icon and color. Deliberately not the nature of
* the change (feature/fix/refactor): groups already partition the PR by intent.
*/
var DiffCategorySchema = /* @__PURE__ */ picklist([
	"ui",
	"api",
	"core",
	"data",
	"cli",
	"security",
	"tests",
	"docs",
	"examples",
	"deps",
	"build",
	"scripts",
	"config",
	"i18n",
	"assets",
	"other"
]);
var CATEGORY_GUIDE = [
	"ui: components, views, styles, layout.",
	"api: endpoints, handlers, contracts between services or packages, third-party integrations.",
	"core: domain/business logic and internal modules; use only when ui/api/data/cli don't fit.",
	"data: schemas, migrations, models, queries, storage.",
	"cli: command-line entry points, arg parsing, terminal output.",
	"security: auth, permissions, secrets handling, input validation.",
	"tests: tests, fixtures, snapshots, stories, benchmarks.",
	"docs: README, guides, changelog, comment-only edits.",
	"examples: examples, playgrounds, demos.",
	"deps: dependency bumps, lockfiles.",
	"build: bundler/compiler/toolchain config, package manifests.",
	"scripts: dev and one-off scripts, automation under bin/ or tools/.",
	"config: runtime config, env, feature flags, linter config, CI/CD, deployment, containers, IaC.",
	"i18n: translations, locales.",
	"assets: images, fonts, static files.",
	"other: generated/vendored code or anything that fits nothing above."
].join(" ");
/**
* Optional explanations the model attaches to a file in its group, or to one line of it.
* Line sides use pierre's vocabulary like review threads do: `additions` lines are
* numbered by the new file, `deletions` lines by the old one.
*/
var noteText = /* @__PURE__ */ pipe(/* @__PURE__ */ string(), /* @__PURE__ */ description("1-2 sentences explaining what a reviewer would otherwise have to work out: non-obvious logic, a subtle behavior change, a risk. Rendered as Markdown."));
var noteCritical = /* @__PURE__ */ optional(/* @__PURE__ */ pipe(/* @__PURE__ */ boolean(), /* @__PURE__ */ description("Set only when the reviewer should take extra care here: security, data loss, hard to revert, easy to get wrong.")));
var FileNoteSchema = /* @__PURE__ */ object({
	path: /* @__PURE__ */ pipe(/* @__PURE__ */ string(), /* @__PURE__ */ description("A path from this group's filePaths.")),
	text: noteText,
	critical: noteCritical
});
var LineNoteSchema = /* @__PURE__ */ object({
	path: /* @__PURE__ */ pipe(/* @__PURE__ */ string(), /* @__PURE__ */ description("A path from this group's filePaths.")),
	side: /* @__PURE__ */ pipe(DiffSideSchema, /* @__PURE__ */ description("\"additions\" for a line that is new or unchanged in the new file, \"deletions\" for a removed line.")),
	line: /* @__PURE__ */ pipe(/* @__PURE__ */ number(), /* @__PURE__ */ description("Line number as counted in the hunk headers: new-file numbering for \"additions\", old-file numbering for \"deletions\".")),
	text: noteText,
	critical: noteCritical
});
/**
* Leaf group shape (no further nesting), reused for both root groups and their children,
* which structurally enforces the "max depth 2" decision rather than relying on convention.
*
* Field `description`s double as the llm adapter's field-level instructions - the model
* sees them directly in the `submit_grouping` tool's parameter schema, so `prompt.ts` only
* needs high-level framing, not a restatement of these per-field rules.
*/
var SubmittedGroupLeafSchema = /* @__PURE__ */ object({
	key: /* @__PURE__ */ pipe(/* @__PURE__ */ string(), /* @__PURE__ */ description("Stable, short, kebab-case-ish id, e.g. \"docs\" or \"feature-a\".")),
	label: /* @__PURE__ */ pipe(/* @__PURE__ */ string(), /* @__PURE__ */ description("Short, human-readable display name for this group.")),
	summary: /* @__PURE__ */ optional(/* @__PURE__ */ pipe(/* @__PURE__ */ string(), /* @__PURE__ */ description("Concise explanation of the intention of this group (why over what). Rendered as Markdown."))),
	category: /* @__PURE__ */ pipe(DiffCategorySchema, /* @__PURE__ */ description(`Which part of the system this group touches. ${CATEGORY_GUIDE}`)),
	filePaths: /* @__PURE__ */ pipe(/* @__PURE__ */ array(/* @__PURE__ */ string()), /* @__PURE__ */ description("File paths belonging directly to this group (not to a child). Every file path given to you MUST end up in exactly one group or child - never both, never omitted.")),
	critical: /* @__PURE__ */ optional(/* @__PURE__ */ pipe(/* @__PURE__ */ boolean(), /* @__PURE__ */ description("Set only when the whole group deserves extra reviewer care (security, data loss, hard to revert). Most groups are not critical."))),
	fileNotes: /* @__PURE__ */ optional(/* @__PURE__ */ pipe(/* @__PURE__ */ array(FileNoteSchema), /* @__PURE__ */ description("Optional notes about a whole file. Add one only where it saves the reviewer time; never explain the obvious. Most files need none."))),
	lineNotes: /* @__PURE__ */ optional(/* @__PURE__ */ pipe(/* @__PURE__ */ array(LineNoteSchema), /* @__PURE__ */ description("Optional notes about one specific line. Same bar as fileNotes; prefer a line note when the point is about one spot in the diff.")))
});
/**
* The stored/shared shape: results cached or shared before `category` existed lack it,
* so only the llm adapter's tool schema (`SubmittedGroupLeafSchema`) requires it. The
* view falls back to 'other'.
*/
var DiffGroupLeafSchema = /* @__PURE__ */ object({
	...SubmittedGroupLeafSchema.entries,
	category: /* @__PURE__ */ optional(SubmittedGroupLeafSchema.entries.category)
});
/** Adds the single allowed nesting level to a leaf shape (depth capped at 2 total: root -> children). */
function withChildren(leaf) {
	return /* @__PURE__ */ object({
		...leaf.entries,
		children: /* @__PURE__ */ optional(/* @__PURE__ */ pipe(/* @__PURE__ */ array(leaf), /* @__PURE__ */ description("Rarely needed; omit by default. One extra level of nesting, only to split a very large group into clearly distinct sub-areas. Children cannot have children of their own.")))
	});
}
var DiffGroupSchema = withChildren(DiffGroupLeafSchema);
({ ...(/* @__PURE__ */ object({
	overallSummary: /* @__PURE__ */ optional(/* @__PURE__ */ pipe(/* @__PURE__ */ string(), /* @__PURE__ */ description("Short paragraph summarizing the whole PR for a reviewer, rendered as Markdown."))),
	groups: /* @__PURE__ */ array(DiffGroupSchema),
	schemaVersion: /* @__PURE__ */ number()
})).entries });
//#endregion
//#region ../../node_modules/.pnpm/@pulls.review+core@0.3.1_typescript@6.0.3_ws@8.22.0_zod@4.6.5/node_modules/@pulls.review/core/dist/rules-CjhczDrM.js
/**
* Generated/lockfile-style output - shared with `noisy-files.ts`, which collapses
* these by default in the diff view, so the two "this file is noise" notions
* (grouping rule vs. default-collapsed) can't drift apart.
*/
var GENERATED_PATTERNS = [
	"**/*.generated.*",
	"**/pnpm-lock.yaml",
	"**/yarn.lock",
	"**/package-lock.json",
	"**/Cargo.lock",
	"**/go.sum",
	"**/dist/**",
	"**/*.lock"
];
//#endregion
//#region ../../node_modules/.pnpm/@pulls.review+core@0.3.1_typescript@6.0.3_ws@8.22.0_zod@4.6.5/node_modules/@pulls.review/core/dist/locales.js
/**
* The languages the UI and the LLM's summaries can be in. `name` is the English name
* the analysis prompt uses (the prompt itself stays English), `native` is what the
* language picker and the shared-analysis banner show.
*/
var LOCALES = [
	{
		code: "en",
		name: "English",
		native: "English"
	},
	{
		code: "zh-CN",
		name: "Simplified Chinese",
		native: "简体中文"
	},
	{
		code: "zh-TW",
		name: "Traditional Chinese",
		native: "繁體中文"
	},
	{
		code: "ja",
		name: "Japanese",
		native: "日本語"
	},
	{
		code: "ko",
		name: "Korean",
		native: "한국어"
	},
	{
		code: "es",
		name: "Spanish",
		native: "Español"
	},
	{
		code: "fr",
		name: "French",
		native: "Français"
	},
	{
		code: "de",
		name: "German",
		native: "Deutsch"
	},
	{
		code: "pt-BR",
		name: "Brazilian Portuguese",
		native: "Português (Brasil)"
	},
	{
		code: "ru",
		name: "Russian",
		native: "Русский"
	},
	{
		code: "it",
		name: "Italian",
		native: "Italiano"
	},
	{
		code: "id",
		name: "Indonesian",
		native: "Bahasa Indonesia"
	}
];
/** How the English prompt names the language to answer in, e.g. `Simplified Chinese (简体中文)`. */
function promptLanguageName(code) {
	const locale = LOCALES.find((entry) => entry.code === code);
	return locale.name === locale.native ? locale.name : `${locale.name} (${locale.native})`;
}
//#endregion
//#region ../../node_modules/.pnpm/picomatch@4.0.7/node_modules/picomatch/lib/constants.js
var require_constants = /* @__PURE__ */ __commonJSMin(((exports, module) => {
	var WIN_SLASH = "\\\\/";
	var WIN_NO_SLASH = `[^${WIN_SLASH}]`;
	var DEFAULT_MAX_EXTGLOB_RECURSION = 0;
	/**
	* Posix glob regex
	*/
	var DOT_LITERAL = "\\.";
	var PLUS_LITERAL = "\\+";
	var QMARK_LITERAL = "\\?";
	var SLASH_LITERAL = "\\/";
	var ONE_CHAR = "(?=.)";
	var QMARK = "[^/]";
	var END_ANCHOR = `(?:${SLASH_LITERAL}|$)`;
	var START_ANCHOR = `(?:^|${SLASH_LITERAL})`;
	var DOTS_SLASH = `${DOT_LITERAL}{1,2}${END_ANCHOR}`;
	var POSIX_CHARS = {
		DOT_LITERAL,
		PLUS_LITERAL,
		QMARK_LITERAL,
		SLASH_LITERAL,
		ONE_CHAR,
		QMARK,
		END_ANCHOR,
		DOTS_SLASH,
		NO_DOT: `(?!${DOT_LITERAL})`,
		NO_DOTS: `(?!${START_ANCHOR}${DOTS_SLASH})`,
		NO_DOT_SLASH: `(?!${DOT_LITERAL}{0,1}${END_ANCHOR})`,
		NO_DOTS_SLASH: `(?!${DOTS_SLASH})`,
		QMARK_NO_DOT: `[^.${SLASH_LITERAL}]`,
		STAR: `${QMARK}*?`,
		START_ANCHOR,
		SEP: "/"
	};
	/**
	* Windows glob regex
	*/
	var WINDOWS_CHARS = {
		...POSIX_CHARS,
		SLASH_LITERAL: `[${WIN_SLASH}]`,
		QMARK: WIN_NO_SLASH,
		STAR: `${WIN_NO_SLASH}*?`,
		DOTS_SLASH: `${DOT_LITERAL}{1,2}(?:[${WIN_SLASH}]|$)`,
		NO_DOT: `(?!${DOT_LITERAL})`,
		NO_DOTS: `(?!(?:^|[${WIN_SLASH}])${DOT_LITERAL}{1,2}(?:[${WIN_SLASH}]|$))`,
		NO_DOT_SLASH: `(?!${DOT_LITERAL}{0,1}(?:[${WIN_SLASH}]|$))`,
		NO_DOTS_SLASH: `(?!${DOT_LITERAL}{1,2}(?:[${WIN_SLASH}]|$))`,
		QMARK_NO_DOT: `[^.${WIN_SLASH}]`,
		START_ANCHOR: `(?:^|[${WIN_SLASH}])`,
		END_ANCHOR: `(?:[${WIN_SLASH}]|$)`,
		SEP: "\\"
	};
	module.exports = {
		DEFAULT_MAX_EXTGLOB_RECURSION,
		MAX_LENGTH: 65536,
		POSIX_REGEX_SOURCE: {
			__proto__: null,
			alnum: "a-zA-Z0-9",
			alpha: "a-zA-Z",
			ascii: "\\x00-\\x7F",
			blank: " \\t",
			cntrl: "\\x00-\\x1F\\x7F",
			digit: "0-9",
			graph: "\\x21-\\x7E",
			lower: "a-z",
			print: "\\x20-\\x7E ",
			punct: "\\-!\"#$%&'()\\*+,./:;<=>?@[\\]^_`{|}~",
			space: " \\t\\r\\n\\v\\f",
			upper: "A-Z",
			word: "A-Za-z0-9_",
			xdigit: "A-Fa-f0-9"
		},
		REGEX_BACKSLASH: /\\(?![*+?^${}(|)[\]])/g,
		REGEX_NON_SPECIAL_CHARS: /^[^@![\].,$*+?^{}()|\\/]+/,
		REGEX_SPECIAL_CHARS: /[-*+?.^${}(|)[\]]/,
		REGEX_SPECIAL_CHARS_BACKREF: /(\\?)((\W)(\3*))/g,
		REGEX_SPECIAL_CHARS_GLOBAL: /([-*+?.^${}(|)[\]])/g,
		REGEX_REMOVE_BACKSLASH: /(?:\[.*?[^\\]\]|\\(?=.))/g,
		REPLACEMENTS: {
			__proto__: null,
			"***": "*",
			"**/**": "**",
			"**/**/**": "**"
		},
		CHAR_0: 48,
		CHAR_9: 57,
		CHAR_UPPERCASE_A: 65,
		CHAR_LOWERCASE_A: 97,
		CHAR_UPPERCASE_Z: 90,
		CHAR_LOWERCASE_Z: 122,
		CHAR_LEFT_PARENTHESES: 40,
		CHAR_RIGHT_PARENTHESES: 41,
		CHAR_ASTERISK: 42,
		CHAR_AMPERSAND: 38,
		CHAR_AT: 64,
		CHAR_BACKWARD_SLASH: 92,
		CHAR_CARRIAGE_RETURN: 13,
		CHAR_CIRCUMFLEX_ACCENT: 94,
		CHAR_COLON: 58,
		CHAR_COMMA: 44,
		CHAR_DOT: 46,
		CHAR_DOUBLE_QUOTE: 34,
		CHAR_EQUAL: 61,
		CHAR_EXCLAMATION_MARK: 33,
		CHAR_FORM_FEED: 12,
		CHAR_FORWARD_SLASH: 47,
		CHAR_GRAVE_ACCENT: 96,
		CHAR_HASH: 35,
		CHAR_HYPHEN_MINUS: 45,
		CHAR_LEFT_ANGLE_BRACKET: 60,
		CHAR_LEFT_CURLY_BRACE: 123,
		CHAR_LEFT_SQUARE_BRACKET: 91,
		CHAR_LINE_FEED: 10,
		CHAR_NO_BREAK_SPACE: 160,
		CHAR_PERCENT: 37,
		CHAR_PLUS: 43,
		CHAR_QUESTION_MARK: 63,
		CHAR_RIGHT_ANGLE_BRACKET: 62,
		CHAR_RIGHT_CURLY_BRACE: 125,
		CHAR_RIGHT_SQUARE_BRACKET: 93,
		CHAR_SEMICOLON: 59,
		CHAR_SINGLE_QUOTE: 39,
		CHAR_SPACE: 32,
		CHAR_TAB: 9,
		CHAR_UNDERSCORE: 95,
		CHAR_VERTICAL_LINE: 124,
		CHAR_ZERO_WIDTH_NOBREAK_SPACE: 65279,
		/**
		* Create EXTGLOB_CHARS
		*/
		extglobChars(chars) {
			return {
				"!": {
					type: "negate",
					open: "(?:(?!(?:",
					close: `))${chars.STAR})`
				},
				"?": {
					type: "qmark",
					open: "(?:",
					close: ")?"
				},
				"+": {
					type: "plus",
					open: "(?:",
					close: ")+"
				},
				"*": {
					type: "star",
					open: "(?:",
					close: ")*"
				},
				"@": {
					type: "at",
					open: "(?:",
					close: ")"
				}
			};
		},
		/**
		* Create GLOB_CHARS
		*/
		globChars(win32) {
			return win32 === true ? WINDOWS_CHARS : POSIX_CHARS;
		}
	};
}));
//#endregion
//#region ../../node_modules/.pnpm/picomatch@4.0.7/node_modules/picomatch/lib/utils.js
var require_utils = /* @__PURE__ */ __commonJSMin(((exports) => {
	var { REGEX_BACKSLASH, REGEX_REMOVE_BACKSLASH, REGEX_SPECIAL_CHARS, REGEX_SPECIAL_CHARS_GLOBAL } = require_constants();
	exports.isObject = (val) => val !== null && typeof val === "object" && !Array.isArray(val);
	exports.hasRegexChars = (str) => REGEX_SPECIAL_CHARS.test(str);
	exports.isRegexChar = (str) => str.length === 1 && exports.hasRegexChars(str);
	exports.escapeRegex = (str) => str.replace(REGEX_SPECIAL_CHARS_GLOBAL, "\\$1");
	exports.toPosixSlashes = (str) => str.replace(REGEX_BACKSLASH, "/");
	exports.isWindows = () => {
		if (typeof navigator !== "undefined" && navigator.platform) {
			const platform = navigator.platform.toLowerCase();
			return platform === "win32" || platform === "windows";
		}
		if (typeof process !== "undefined" && process.platform) return process.platform === "win32";
		return false;
	};
	exports.removeBackslashes = (str) => {
		return str.replace(REGEX_REMOVE_BACKSLASH, (match) => {
			return match === "\\" ? "" : match;
		});
	};
	exports.escapeLast = (input, char, lastIdx) => {
		const idx = input.lastIndexOf(char, lastIdx);
		if (idx === -1) return input;
		if (input[idx - 1] === "\\") return exports.escapeLast(input, char, idx - 1);
		return `${input.slice(0, idx)}\\${input.slice(idx)}`;
	};
	exports.removePrefix = (input, state = {}) => {
		let output = input;
		if (output.startsWith("./")) {
			output = output.slice(2);
			state.prefix = "./";
		}
		return output;
	};
	exports.wrapOutput = (input, state = {}, options = {}) => {
		let output = `${options.contains ? "" : "^"}(?:${input})${options.contains ? "" : "$"}`;
		if (state.negated === true) output = `(?:^(?!${output}).*$)`;
		return output;
	};
	exports.basename = (path, { windows } = {}) => {
		const segs = path.split(windows ? /[\\/]/ : "/");
		const last = segs[segs.length - 1];
		if (last === "") return segs[segs.length - 2];
		return last;
	};
}));
//#endregion
//#region ../../node_modules/.pnpm/picomatch@4.0.7/node_modules/picomatch/lib/scan.js
var require_scan = /* @__PURE__ */ __commonJSMin(((exports, module) => {
	var utils = require_utils();
	var { CHAR_ASTERISK, CHAR_AT, CHAR_BACKWARD_SLASH, CHAR_COMMA, CHAR_DOT, CHAR_EXCLAMATION_MARK, CHAR_FORWARD_SLASH, CHAR_LEFT_CURLY_BRACE, CHAR_LEFT_PARENTHESES, CHAR_LEFT_SQUARE_BRACKET, CHAR_PLUS, CHAR_QUESTION_MARK, CHAR_RIGHT_CURLY_BRACE, CHAR_RIGHT_PARENTHESES, CHAR_RIGHT_SQUARE_BRACKET } = require_constants();
	var isPathSeparator = (code) => {
		return code === CHAR_FORWARD_SLASH || code === CHAR_BACKWARD_SLASH;
	};
	var depth = (token) => {
		if (token.isPrefix !== true) token.depth = token.isGlobstar ? Infinity : 1;
	};
	/**
	* Quickly scans a glob pattern and returns an object with a handful of
	* useful properties, like `isGlob`, `path` (the leading non-glob, if it exists),
	* `glob` (the actual pattern), `negated` (true if the path starts with `!` but not
	* with `!(`) and `negatedExtglob` (true if the path starts with `!(`).
	*
	* ```js
	* const pm = require('picomatch');
	* console.log(pm.scan('foo/bar/*.js'));
	* { isGlob: true, input: 'foo/bar/*.js', base: 'foo/bar', glob: '*.js' }
	* ```
	* @param {String} `str`
	* @param {Object} `options`
	* @return {Object} Returns an object with tokens and regex source string.
	* @api public
	*/
	var scan = (input, options) => {
		const opts = options || {};
		const length = input.length - 1;
		const scanToEnd = opts.parts === true || opts.tokens === true || opts.scanToEnd === true;
		const slashes = [];
		const tokens = [];
		const parts = [];
		let str = input;
		let index = -1;
		let start = 0;
		let lastIndex = 0;
		let isBrace = false;
		let isBracket = false;
		let isGlob = false;
		let isExtglob = false;
		let isGlobstar = false;
		let braceEscaped = false;
		let backslashes = false;
		let negated = false;
		let negatedExtglob = false;
		let finished = false;
		let braces = 0;
		let prev;
		let code;
		let token = {
			value: "",
			depth: 0,
			isGlob: false
		};
		const eos = () => index >= length;
		const peek = () => str.charCodeAt(index + 1);
		const advance = () => {
			prev = code;
			return str.charCodeAt(++index);
		};
		while (index < length) {
			code = advance();
			let next;
			if (code === CHAR_BACKWARD_SLASH) {
				backslashes = token.backslashes = true;
				code = advance();
				if (code === CHAR_LEFT_CURLY_BRACE) braceEscaped = true;
				continue;
			}
			if (braceEscaped === true || code === CHAR_LEFT_CURLY_BRACE) {
				braces++;
				while (eos() !== true && (code = advance())) {
					if (code === CHAR_BACKWARD_SLASH) {
						backslashes = token.backslashes = true;
						advance();
						continue;
					}
					if (code === CHAR_LEFT_CURLY_BRACE) {
						braces++;
						continue;
					}
					if (braceEscaped !== true && code === CHAR_DOT && (code = advance()) === CHAR_DOT) {
						isBrace = token.isBrace = true;
						isGlob = token.isGlob = true;
						finished = true;
						if (scanToEnd === true) continue;
						break;
					}
					if (braceEscaped !== true && code === CHAR_COMMA) {
						isBrace = token.isBrace = true;
						isGlob = token.isGlob = true;
						finished = true;
						if (scanToEnd === true) continue;
						break;
					}
					if (code === CHAR_RIGHT_CURLY_BRACE) {
						braces--;
						if (braces === 0) {
							braceEscaped = false;
							isBrace = token.isBrace = true;
							finished = true;
							break;
						}
					}
				}
				if (scanToEnd === true) continue;
				break;
			}
			if (code === CHAR_FORWARD_SLASH) {
				slashes.push(index);
				tokens.push(token);
				token = {
					value: "",
					depth: 0,
					isGlob: false
				};
				if (finished === true) continue;
				if (prev === CHAR_DOT && index === start + 1) {
					start += 2;
					continue;
				}
				lastIndex = index + 1;
				continue;
			}
			if (opts.noext !== true) {
				if ((code === CHAR_PLUS || code === CHAR_AT || code === CHAR_ASTERISK || code === CHAR_QUESTION_MARK || code === CHAR_EXCLAMATION_MARK) === true && peek() === CHAR_LEFT_PARENTHESES) {
					isGlob = token.isGlob = true;
					isExtglob = token.isExtglob = true;
					finished = true;
					if (code === CHAR_EXCLAMATION_MARK && index === start) negatedExtglob = true;
					if (scanToEnd === true) {
						let parens = 0;
						while (eos() !== true && (code = advance())) {
							if (code === CHAR_BACKWARD_SLASH) {
								backslashes = token.backslashes = true;
								advance();
								continue;
							}
							if (code === CHAR_LEFT_PARENTHESES) {
								parens++;
								continue;
							}
							if (code === CHAR_RIGHT_PARENTHESES && --parens === 0) {
								finished = true;
								break;
							}
						}
						continue;
					}
					break;
				}
			}
			if (code === CHAR_ASTERISK) {
				if (prev === CHAR_ASTERISK) isGlobstar = token.isGlobstar = true;
				isGlob = token.isGlob = true;
				finished = true;
				if (scanToEnd === true) continue;
				break;
			}
			if (code === CHAR_QUESTION_MARK) {
				isGlob = token.isGlob = true;
				finished = true;
				if (scanToEnd === true) continue;
				break;
			}
			if (code === CHAR_LEFT_SQUARE_BRACKET) {
				while (eos() !== true && (next = advance())) {
					if (next === CHAR_BACKWARD_SLASH) {
						backslashes = token.backslashes = true;
						advance();
						continue;
					}
					if (next === CHAR_RIGHT_SQUARE_BRACKET) {
						isBracket = token.isBracket = true;
						isGlob = token.isGlob = true;
						finished = true;
						break;
					}
				}
				if (scanToEnd === true) continue;
				break;
			}
			if (opts.nonegate !== true && code === CHAR_EXCLAMATION_MARK && index === start) {
				negated = token.negated = true;
				start++;
				continue;
			}
			if (opts.noparen !== true && code === CHAR_LEFT_PARENTHESES) {
				isGlob = token.isGlob = true;
				if (scanToEnd === true) {
					let parens = 1;
					while (eos() !== true && (code = advance())) {
						if (code === CHAR_BACKWARD_SLASH) {
							backslashes = token.backslashes = true;
							advance();
							continue;
						}
						if (code === CHAR_LEFT_PARENTHESES) {
							parens++;
							continue;
						}
						if (code === CHAR_RIGHT_PARENTHESES && --parens === 0) {
							finished = true;
							break;
						}
					}
					continue;
				}
				break;
			}
			if (isGlob === true) {
				finished = true;
				if (scanToEnd === true) continue;
				break;
			}
		}
		if (opts.noext === true) {
			isExtglob = false;
			isGlob = false;
		}
		let base = str;
		let prefix = "";
		let glob = "";
		if (start > 0) {
			prefix = str.slice(0, start);
			str = str.slice(start);
			lastIndex -= start;
		}
		if (base && isGlob === true && lastIndex > 0) {
			base = str.slice(0, lastIndex);
			glob = str.slice(lastIndex);
		} else if (isGlob === true) {
			base = "";
			glob = str;
		} else base = str;
		if (base && base !== "" && base !== "/" && base !== str) {
			if (isPathSeparator(base.charCodeAt(base.length - 1))) base = base.slice(0, -1);
		}
		if (opts.unescape === true) {
			if (glob) glob = utils.removeBackslashes(glob);
			if (base && backslashes === true) base = utils.removeBackslashes(base);
		}
		const state = {
			prefix,
			input,
			start,
			base,
			glob,
			isBrace,
			isBracket,
			isGlob,
			isExtglob,
			isGlobstar,
			negated,
			negatedExtglob
		};
		if (opts.tokens === true) {
			state.maxDepth = 0;
			if (!isPathSeparator(code)) tokens.push(token);
			state.tokens = tokens;
		}
		if (opts.parts === true || opts.tokens === true) {
			let prevIndex;
			for (let idx = 0; idx < slashes.length; idx++) {
				const n = prevIndex !== void 0 ? prevIndex + 1 : start;
				const i = slashes[idx];
				const value = input.slice(n, i);
				if (opts.tokens) {
					if (idx === 0 && start !== 0) {
						tokens[idx].isPrefix = true;
						tokens[idx].value = prefix;
					} else tokens[idx].value = value;
					depth(tokens[idx]);
					state.maxDepth += tokens[idx].depth;
				}
				if (i >= start) {
					parts.push(value);
					prevIndex = i;
				}
			}
			const n = prevIndex !== void 0 ? prevIndex + 1 : start;
			const value = input.slice(n);
			parts.push(value);
			if (opts.tokens && prevIndex && prevIndex + 1 < input.length) {
				tokens[tokens.length - 1].value = value;
				depth(tokens[tokens.length - 1]);
				state.maxDepth += tokens[tokens.length - 1].depth;
			}
			state.slashes = slashes;
			state.parts = parts;
		}
		return state;
	};
	module.exports = scan;
}));
//#endregion
//#region ../../node_modules/.pnpm/picomatch@4.0.7/node_modules/picomatch/lib/parse.js
var require_parse = /* @__PURE__ */ __commonJSMin(((exports, module) => {
	var constants = require_constants();
	var utils = require_utils();
	/**
	* Constants
	*/
	var { MAX_LENGTH, POSIX_REGEX_SOURCE, REGEX_NON_SPECIAL_CHARS, REGEX_SPECIAL_CHARS_BACKREF, REPLACEMENTS } = constants;
	/**
	* Helpers
	*/
	var expandRange = (args, options) => {
		if (typeof options.expandRange === "function") return options.expandRange(...args, options);
		args.sort();
		const value = `[${args.join("-")}]`;
		try {
			new RegExp(value);
		} catch (ex) {
			return args.map((v) => utils.escapeRegex(v)).join("..");
		}
		return value;
	};
	/**
	* Create the message for a syntax error
	*/
	var syntaxError = (type, char) => {
		return `Missing ${type}: "${char}" - use "\\\\${char}" to match literal characters`;
	};
	var splitTopLevel = (input) => {
		const parts = [];
		let bracket = 0;
		let paren = 0;
		let quote = 0;
		let value = "";
		let escaped = false;
		for (const ch of input) {
			if (escaped === true) {
				value += ch;
				escaped = false;
				continue;
			}
			if (ch === "\\") {
				value += ch;
				escaped = true;
				continue;
			}
			if (ch === "\"") {
				quote = quote === 1 ? 0 : 1;
				value += ch;
				continue;
			}
			if (quote === 0) {
				if (ch === "[") bracket++;
				else if (ch === "]" && bracket > 0) bracket--;
				else if (bracket === 0) {
					if (ch === "(") paren++;
					else if (ch === ")" && paren > 0) paren--;
					else if (ch === "|" && paren === 0) {
						parts.push(value);
						value = "";
						continue;
					}
				}
			}
			value += ch;
		}
		parts.push(value);
		return parts;
	};
	var isPlainBranch = (branch) => {
		let escaped = false;
		for (const ch of branch) {
			if (escaped === true) {
				escaped = false;
				continue;
			}
			if (ch === "\\") {
				escaped = true;
				continue;
			}
			if (/[?*+@!()[\]{}]/.test(ch)) return false;
		}
		return true;
	};
	var normalizeSimpleBranch = (branch) => {
		let value = branch.trim();
		let changed = true;
		while (changed === true) {
			changed = false;
			if (/^@\([^\\()[\]{}|]+\)$/.test(value)) {
				value = value.slice(2, -1);
				changed = true;
			}
		}
		if (!isPlainBranch(value)) return;
		return value.replace(/\\(.)/g, "$1");
	};
	var hasRepeatedCharPrefixOverlap = (branches) => {
		const values = branches.map(normalizeSimpleBranch).filter(Boolean);
		for (let i = 0; i < values.length; i++) for (let j = i + 1; j < values.length; j++) {
			const a = values[i];
			const b = values[j];
			const char = a[0];
			if (!char || a !== char.repeat(a.length) || b !== char.repeat(b.length)) continue;
			if (a === b || a.startsWith(b) || b.startsWith(a)) return true;
		}
		return false;
	};
	var parseRepeatedExtglob = (pattern, requireEnd = true) => {
		if (pattern[0] !== "+" && pattern[0] !== "*" || pattern[1] !== "(") return;
		let bracket = 0;
		let paren = 0;
		let quote = 0;
		let escaped = false;
		for (let i = 1; i < pattern.length; i++) {
			const ch = pattern[i];
			if (escaped === true) {
				escaped = false;
				continue;
			}
			if (ch === "\\") {
				escaped = true;
				continue;
			}
			if (ch === "\"") {
				quote = quote === 1 ? 0 : 1;
				continue;
			}
			if (quote === 1) continue;
			if (ch === "[") {
				bracket++;
				continue;
			}
			if (ch === "]" && bracket > 0) {
				bracket--;
				continue;
			}
			if (bracket > 0) continue;
			if (ch === "(") {
				paren++;
				continue;
			}
			if (ch === ")") {
				paren--;
				if (paren === 0) {
					if (requireEnd === true && i !== pattern.length - 1) return;
					return {
						type: pattern[0],
						body: pattern.slice(2, i),
						end: i
					};
				}
			}
		}
	};
	var buildCharClassStar = (chars) => {
		return `${chars.length === 1 ? utils.escapeRegex(chars[0]) : `[${chars.map((ch) => utils.escapeRegex(ch)).join("")}]`}*`;
	};
	var getStarExtglobSequenceChars = (pattern) => {
		let index = 0;
		const chars = [];
		while (index < pattern.length) {
			const match = parseRepeatedExtglob(pattern.slice(index), false);
			if (!match || match.type !== "*") return;
			const branches = splitTopLevel(match.body).map((branch) => branch.trim());
			if (branches.length !== 1) return;
			const branch = normalizeSimpleBranch(branches[0]);
			if (!branch || branch.length !== 1) return;
			chars.push(branch);
			index += match.end + 1;
		}
		if (chars.length < 1) return;
		return chars;
	};
	var repeatedExtglobRecursion = (pattern) => {
		let depth = 0;
		let value = pattern.trim();
		let match = parseRepeatedExtglob(value);
		while (match) {
			depth++;
			value = match.body.trim();
			match = parseRepeatedExtglob(value);
		}
		return depth;
	};
	var analyzeRepeatedExtglob = (body, options) => {
		if (options.maxExtglobRecursion === false) return { risky: false };
		const max = typeof options.maxExtglobRecursion === "number" ? options.maxExtglobRecursion : constants.DEFAULT_MAX_EXTGLOB_RECURSION;
		const branches = splitTopLevel(body).map((branch) => branch.trim());
		if (branches.length > 1) {
			if (branches.some((branch) => branch === "") || branches.some((branch) => /^[*?]+$/.test(branch)) || hasRepeatedCharPrefixOverlap(branches)) return { risky: true };
		}
		const safeChars = [];
		let sawStarSequence = false;
		let combinable = true;
		for (const branch of branches) {
			const chars = getStarExtglobSequenceChars(branch);
			if (chars) {
				sawStarSequence = true;
				safeChars.push(...chars);
				continue;
			}
			const literal = normalizeSimpleBranch(branch);
			if (literal && literal.length === 1) {
				safeChars.push(literal);
				continue;
			}
			combinable = false;
			if (repeatedExtglobRecursion(branch) > max) return { risky: true };
		}
		if (sawStarSequence) return combinable ? {
			risky: true,
			safeOutput: buildCharClassStar([...new Set(safeChars)])
		} : { risky: true };
		return { risky: false };
	};
	/**
	* Parse the given input string.
	* @param {String} input
	* @param {Object} options
	* @return {Object}
	*/
	var parse = (input, options) => {
		if (typeof input !== "string") throw new TypeError("Expected a string");
		input = REPLACEMENTS[input] || input;
		const opts = { ...options };
		const max = typeof opts.maxLength === "number" ? Math.min(MAX_LENGTH, opts.maxLength) : MAX_LENGTH;
		let len = input.length;
		if (len > max) throw new SyntaxError(`Input length: ${len}, exceeds maximum allowed length: ${max}`);
		const bos = {
			type: "bos",
			value: "",
			output: opts.prepend || ""
		};
		const tokens = [bos];
		const capture = opts.capture ? "" : "?:";
		const PLATFORM_CHARS = constants.globChars(opts.windows);
		const EXTGLOB_CHARS = constants.extglobChars(PLATFORM_CHARS);
		const { DOT_LITERAL, PLUS_LITERAL, SLASH_LITERAL, ONE_CHAR, DOTS_SLASH, NO_DOT, NO_DOT_SLASH, NO_DOTS_SLASH, QMARK, QMARK_NO_DOT, STAR, START_ANCHOR } = PLATFORM_CHARS;
		const globstar = (opts) => {
			return `(${capture}(?:(?!${START_ANCHOR}${opts.dot ? DOTS_SLASH : DOT_LITERAL}).)*?)`;
		};
		const nodot = opts.dot ? "" : NO_DOT;
		const qmarkNoDot = opts.dot ? QMARK : QMARK_NO_DOT;
		let star = opts.bash === true ? globstar(opts) : STAR;
		if (opts.capture) star = `(${star})`;
		if (typeof opts.noext === "boolean") opts.noextglob = opts.noext;
		const state = {
			input,
			index: -1,
			start: 0,
			dot: opts.dot === true,
			consumed: "",
			output: "",
			prefix: "",
			backtrack: false,
			negated: false,
			brackets: 0,
			braces: 0,
			parens: 0,
			quotes: 0,
			globstar: false,
			tokens
		};
		input = utils.removePrefix(input, state);
		len = input.length;
		const extglobs = [];
		const braces = [];
		const stack = [];
		let prev = bos;
		let value;
		/**
		* Tokenizing helpers
		*/
		const eos = () => state.index === len - 1;
		const peek = state.peek = (n = 1) => input[state.index + n];
		const advance = state.advance = () => input[++state.index] || "";
		const remaining = () => input.slice(state.index + 1);
		const consume = (value = "", num = 0) => {
			state.consumed += value;
			state.index += num;
		};
		const append = (token) => {
			state.output += token.output != null ? token.output : token.value;
			consume(token.value);
		};
		const negate = () => {
			let count = 1;
			while (peek() === "!" && (peek(2) !== "(" || peek(3) === "?")) {
				advance();
				state.start++;
				count++;
			}
			if (count % 2 === 0) return false;
			state.negated = true;
			state.start++;
			return true;
		};
		const increment = (type) => {
			state[type]++;
			stack.push(type);
		};
		const decrement = (type) => {
			state[type]--;
			stack.pop();
		};
		/**
		* Push tokens onto the tokens array. This helper speeds up
		* tokenizing by 1) helping us avoid backtracking as much as possible,
		* and 2) helping us avoid creating extra tokens when consecutive
		* characters are plain text. This improves performance and simplifies
		* lookbehinds.
		*/
		const push = (tok) => {
			if (prev.type === "globstar") {
				const isBrace = state.braces > 0 && (tok.type === "comma" || tok.type === "brace");
				const isExtglob = tok.extglob === true || extglobs.length && (tok.type === "pipe" || tok.type === "paren");
				if (tok.type !== "slash" && tok.type !== "paren" && !isBrace && !isExtglob) {
					state.output = state.output.slice(0, -prev.output.length);
					prev.type = "star";
					prev.value = "*";
					prev.output = star;
					state.output += prev.output;
				}
			}
			if (extglobs.length && tok.type !== "paren") extglobs[extglobs.length - 1].inner += tok.value;
			if (tok.value || tok.output) append(tok);
			if (prev && prev.type === "text" && tok.type === "text") {
				prev.output = (prev.output || prev.value) + tok.value;
				prev.value += tok.value;
				return;
			}
			tok.prev = prev;
			tokens.push(tok);
			prev = tok;
		};
		const extglobOpen = (type, value) => {
			const token = {
				...EXTGLOB_CHARS[value],
				conditions: 1,
				inner: ""
			};
			token.prev = prev;
			token.parens = state.parens;
			token.output = state.output;
			token.startIndex = state.index;
			token.tokensIndex = tokens.length;
			const output = (opts.capture ? "(" : "") + token.open;
			increment("parens");
			push({
				type,
				value,
				output: state.output ? "" : ONE_CHAR
			});
			push({
				type: "paren",
				extglob: true,
				value: advance(),
				output
			});
			extglobs.push(token);
		};
		const extglobClose = (token) => {
			const literal = input.slice(token.startIndex, state.index + 1);
			const analysis = analyzeRepeatedExtglob(input.slice(token.startIndex + 2, state.index), opts);
			if ((token.type === "plus" || token.type === "star") && analysis.risky) {
				const safeOutput = analysis.safeOutput ? (token.output ? "" : ONE_CHAR) + (opts.capture ? `(${analysis.safeOutput})` : analysis.safeOutput) : void 0;
				const open = tokens[token.tokensIndex];
				open.type = "text";
				open.value = literal;
				open.output = safeOutput || utils.escapeRegex(literal);
				for (let i = token.tokensIndex + 1; i < tokens.length; i++) {
					tokens[i].value = "";
					tokens[i].output = "";
					delete tokens[i].suffix;
				}
				state.output = token.output + open.output;
				state.backtrack = true;
				push({
					type: "paren",
					extglob: true,
					value,
					output: ""
				});
				decrement("parens");
				return;
			}
			let output = token.close + (opts.capture ? ")" : "");
			let rest;
			if (token.type === "negate") {
				let extglobStar = star;
				if (token.inner && token.inner.length > 1 && token.inner.includes("/")) extglobStar = globstar(opts);
				if (extglobStar !== star || eos() || /^\)+$/.test(remaining())) output = token.close = `)$))${extglobStar}`;
				if (token.inner.includes("*") && (rest = remaining()) && /^\.[^\\/.]+$/.test(rest)) output = token.close = `)${parse(rest, {
					...options,
					fastpaths: false
				}).output})${extglobStar})`;
				if (token.prev.type === "bos") state.negatedExtglob = true;
			}
			push({
				type: "paren",
				extglob: true,
				value,
				output
			});
			decrement("parens");
		};
		/**
		* Fast paths
		*/
		if (opts.fastpaths !== false && !/(^[*!]|[/()[\]{}"])/.test(input)) {
			let backslashes = false;
			let output = input.replace(REGEX_SPECIAL_CHARS_BACKREF, (m, esc, chars, first, rest, index) => {
				if (first === "\\") {
					backslashes = true;
					return m;
				}
				if (first === "?") {
					if (esc) return esc + first + (rest ? QMARK.repeat(rest.length) : "");
					if (index === 0) return qmarkNoDot + (rest ? QMARK.repeat(rest.length) : "");
					return QMARK.repeat(chars.length);
				}
				if (first === ".") return DOT_LITERAL.repeat(chars.length);
				if (first === "*") {
					if (esc) return esc + first + (rest ? star : "");
					return star;
				}
				return esc ? m : `\\${m}`;
			});
			if (backslashes === true) {
				if (opts.unescape === true) output = output.replace(/\\/g, "");
				else output = output.replace(/\\+/g, (m) => {
					return m.length % 2 === 0 ? "\\\\" : m ? "\\" : "";
				});
			}
			if (output === input && opts.contains === true) {
				state.output = input;
				return state;
			}
			state.output = utils.wrapOutput(output, state, options);
			return state;
		}
		/**
		* Tokenize input until we reach end-of-string
		*/
		while (!eos()) {
			value = advance();
			if (value === "\0") continue;
			/**
			* Escaped characters
			*/
			if (value === "\\") {
				const next = peek();
				if (next === "/" && opts.bash !== true) continue;
				if (next === "." || next === ";") continue;
				if (!next) {
					value += "\\";
					push({
						type: "text",
						value
					});
					continue;
				}
				const match = /^\\+/.exec(remaining());
				let slashes = 0;
				if (match && match[0].length > 2) {
					slashes = match[0].length;
					state.index += slashes;
					if (slashes % 2 !== 0) value += "\\";
				}
				if (opts.unescape === true) value = advance();
				else value += advance();
				if (state.brackets === 0) {
					push({
						type: "text",
						value
					});
					continue;
				}
			}
			/**
			* If we're inside a regex character class, continue
			* until we reach the closing bracket.
			*/
			if (state.brackets > 0 && (value !== "]" || prev.value === "[" || prev.value === "[^")) {
				if (opts.posix !== false && value === ":") {
					const inner = prev.value.slice(1);
					if (inner.includes("[")) {
						prev.posix = true;
						if (inner.includes(":")) {
							const idx = prev.value.lastIndexOf("[");
							const pre = prev.value.slice(0, idx);
							const posix = POSIX_REGEX_SOURCE[prev.value.slice(idx + 2)];
							if (posix) {
								prev.value = pre + posix;
								state.backtrack = true;
								advance();
								if (!bos.output && tokens.indexOf(prev) === 1) bos.output = ONE_CHAR;
								continue;
							}
						}
					}
				}
				if (value === "[" && peek() !== ":" || value === "-" && peek() === "]") value = `\\${value}`;
				if (value === "]" && (prev.value === "[" || prev.value === "[^")) value = `\\${value}`;
				if (opts.posix === true && value === "!" && prev.value === "[") value = "^";
				prev.value += value;
				append({ value });
				continue;
			}
			/**
			* If we're inside a quoted string, continue
			* until we reach the closing double quote.
			*/
			if (state.quotes === 1 && value !== "\"") {
				value = utils.escapeRegex(value);
				prev.value += value;
				append({ value });
				continue;
			}
			/**
			* Double quotes
			*/
			if (value === "\"") {
				state.quotes = state.quotes === 1 ? 0 : 1;
				if (opts.keepQuotes === true) push({
					type: "text",
					value
				});
				continue;
			}
			/**
			* Parentheses
			*/
			if (value === "(") {
				increment("parens");
				push({
					type: "paren",
					value
				});
				continue;
			}
			if (value === ")") {
				if (state.parens === 0 && opts.strictBrackets === true) throw new SyntaxError(syntaxError("opening", "("));
				const extglob = extglobs[extglobs.length - 1];
				if (extglob && state.parens === extglob.parens + 1) {
					extglobClose(extglobs.pop());
					continue;
				}
				push({
					type: "paren",
					value,
					output: state.parens ? ")" : "\\)"
				});
				decrement("parens");
				continue;
			}
			/**
			* Square brackets
			*/
			if (value === "[") {
				if (opts.nobracket === true || !remaining().includes("]")) {
					if (opts.nobracket !== true && opts.strictBrackets === true) throw new SyntaxError(syntaxError("closing", "]"));
					value = `\\${value}`;
				} else increment("brackets");
				push({
					type: "bracket",
					value
				});
				continue;
			}
			if (value === "]") {
				if (opts.nobracket === true || prev && prev.type === "bracket" && prev.value.length === 1) {
					push({
						type: "text",
						value,
						output: `\\${value}`
					});
					continue;
				}
				if (state.brackets === 0) {
					if (opts.strictBrackets === true) throw new SyntaxError(syntaxError("opening", "["));
					push({
						type: "text",
						value,
						output: `\\${value}`
					});
					continue;
				}
				decrement("brackets");
				const prevValue = prev.value.slice(1);
				if (prev.posix !== true && prevValue[0] === "^" && !prevValue.includes("/")) value = `/${value}`;
				prev.value += value;
				append({ value });
				if (opts.literalBrackets === false || utils.hasRegexChars(prevValue)) continue;
				const escaped = utils.escapeRegex(prev.value);
				state.output = state.output.slice(0, -prev.value.length);
				if (opts.literalBrackets === true) {
					state.output += escaped;
					prev.value = escaped;
					continue;
				}
				prev.value = `(${capture}${escaped}|${prev.value})`;
				state.output += prev.value;
				continue;
			}
			/**
			* Braces
			*/
			if (value === "{" && opts.nobrace !== true) {
				increment("braces");
				const open = {
					type: "brace",
					value,
					output: "(",
					outputIndex: state.output.length,
					tokensIndex: state.tokens.length
				};
				braces.push(open);
				push(open);
				continue;
			}
			if (value === "}") {
				const brace = braces[braces.length - 1];
				if (opts.nobrace === true || !brace) {
					push({
						type: "text",
						value,
						output: value
					});
					continue;
				}
				let output = ")";
				if (brace.dots === true) {
					const arr = tokens.slice();
					const range = [];
					for (let i = arr.length - 1; i >= 0; i--) {
						tokens.pop();
						if (arr[i].type === "brace") break;
						if (arr[i].type !== "dots") range.unshift(arr[i].value);
					}
					output = expandRange(range, opts);
					state.backtrack = true;
				}
				if (brace.comma !== true && brace.dots !== true) {
					const out = state.output.slice(0, brace.outputIndex);
					const toks = state.tokens.slice(brace.tokensIndex);
					brace.value = brace.output = "\\{";
					value = output = "\\}";
					state.output = out;
					for (const t of toks) state.output += t.output || t.value;
				}
				push({
					type: "brace",
					value,
					output
				});
				decrement("braces");
				braces.pop();
				continue;
			}
			/**
			* Pipes
			*/
			if (value === "|") {
				if (extglobs.length > 0) extglobs[extglobs.length - 1].conditions++;
				push({
					type: "text",
					value
				});
				continue;
			}
			/**
			* Commas
			*/
			if (value === ",") {
				let output = value;
				const brace = braces[braces.length - 1];
				if (brace && stack[stack.length - 1] === "braces") {
					brace.comma = true;
					output = "|";
				}
				push({
					type: "comma",
					value,
					output
				});
				continue;
			}
			/**
			* Slashes
			*/
			if (value === "/") {
				if (prev.type === "dot" && state.index === state.start + 1) {
					state.start = state.index + 1;
					state.consumed = "";
					state.output = "";
					tokens.pop();
					prev = bos;
					continue;
				}
				push({
					type: "slash",
					value,
					output: SLASH_LITERAL
				});
				continue;
			}
			/**
			* Dots
			*/
			if (value === ".") {
				if (state.braces > 0 && prev.type === "dot") {
					if (prev.value === ".") prev.output = DOT_LITERAL;
					const brace = braces[braces.length - 1];
					prev.type = "dots";
					prev.output += value;
					prev.value += value;
					brace.dots = true;
					continue;
				}
				if (state.braces + state.parens === 0 && prev.type !== "bos" && prev.type !== "slash") {
					push({
						type: "text",
						value,
						output: DOT_LITERAL
					});
					continue;
				}
				push({
					type: "dot",
					value,
					output: DOT_LITERAL
				});
				continue;
			}
			/**
			* Question marks
			*/
			if (value === "?") {
				if (!(prev && prev.value === "(") && opts.noextglob !== true && peek() === "(" && peek(2) !== "?") {
					extglobOpen("qmark", value);
					continue;
				}
				if (prev && prev.type === "paren") {
					const next = peek();
					let output = value;
					if (prev.value === "(" && !/[!=<:]/.test(next) || next === "<" && !/<([!=]|\w+>)/.test(remaining())) output = `\\${value}`;
					push({
						type: "text",
						value,
						output
					});
					continue;
				}
				if (opts.dot !== true && (prev.type === "slash" || prev.type === "bos")) {
					push({
						type: "qmark",
						value,
						output: QMARK_NO_DOT
					});
					continue;
				}
				push({
					type: "qmark",
					value,
					output: QMARK
				});
				continue;
			}
			/**
			* Exclamation
			*/
			if (value === "!") {
				if (opts.noextglob !== true && peek() === "(") {
					if (peek(2) !== "?" || !/[!=<:]/.test(peek(3))) {
						extglobOpen("negate", value);
						continue;
					}
				}
				if (opts.nonegate !== true && state.index === 0) {
					negate();
					continue;
				}
			}
			/**
			* Plus
			*/
			if (value === "+") {
				if (opts.noextglob !== true && peek() === "(" && peek(2) !== "?") {
					extglobOpen("plus", value);
					continue;
				}
				if (prev && prev.value === "(" || opts.regex === false) {
					push({
						type: "plus",
						value,
						output: PLUS_LITERAL
					});
					continue;
				}
				if (prev && (prev.type === "bracket" || prev.type === "paren" || prev.type === "brace") || state.parens > 0) {
					push({
						type: "plus",
						value
					});
					continue;
				}
				push({
					type: "plus",
					value: PLUS_LITERAL
				});
				continue;
			}
			/**
			* Plain text
			*/
			if (value === "@") {
				if (opts.noextglob !== true && peek() === "(" && peek(2) !== "?") {
					push({
						type: "at",
						extglob: true,
						value,
						output: ""
					});
					continue;
				}
				push({
					type: "text",
					value
				});
				continue;
			}
			/**
			* Plain text
			*/
			if (value !== "*") {
				if (value === "$" || value === "^") value = `\\${value}`;
				const match = REGEX_NON_SPECIAL_CHARS.exec(remaining());
				if (match) {
					value += match[0];
					state.index += match[0].length;
				}
				push({
					type: "text",
					value
				});
				continue;
			}
			/**
			* Stars
			*/
			if (prev && (prev.type === "globstar" || prev.star === true)) {
				prev.type = "star";
				prev.star = true;
				prev.value += value;
				prev.output = star;
				state.backtrack = true;
				state.globstar = true;
				consume(value);
				continue;
			}
			let rest = remaining();
			if (opts.noextglob !== true && /^\([^?]/.test(rest)) {
				extglobOpen("star", value);
				continue;
			}
			if (prev.type === "star") {
				if (opts.noglobstar === true) {
					consume(value);
					continue;
				}
				const prior = prev.prev;
				const before = prior.prev;
				const isStart = prior.type === "slash" || prior.type === "bos";
				const afterStar = before && (before.type === "star" || before.type === "globstar");
				if (opts.bash === true && (!isStart || rest[0] && rest[0] !== "/")) {
					push({
						type: "star",
						value,
						output: ""
					});
					continue;
				}
				const isBrace = state.braces > 0 && (prior.type === "comma" || prior.type === "brace");
				const isExtglob = extglobs.length && (prior.type === "pipe" || prior.type === "paren");
				if (!isStart && prior.type !== "paren" && !isBrace && !isExtglob) {
					push({
						type: "star",
						value,
						output: ""
					});
					continue;
				}
				while (rest.slice(0, 3) === "/**") {
					const after = input[state.index + 4];
					if (after && after !== "/") break;
					rest = rest.slice(3);
					consume("/**", 3);
				}
				const isEnd = eos() || state.parens > 0 && rest === ")".repeat(state.parens) && !extglobs.some((extglob) => extglob.type === "negate");
				if (prior.type === "bos" && eos()) {
					prev.type = "globstar";
					prev.value += value;
					prev.output = globstar(opts);
					state.output = prev.output;
					state.globstar = true;
					consume(value);
					continue;
				}
				if (prior.type === "slash" && prior.prev.type !== "bos" && !afterStar && isEnd) {
					state.output = state.output.slice(0, -(prior.output + prev.output).length);
					prior.output = `(?:${prior.output}`;
					prev.type = "globstar";
					prev.output = globstar(opts) + (opts.strictSlashes ? ")" : "|$)");
					prev.value += value;
					state.globstar = true;
					state.output += prior.output + prev.output;
					consume(value);
					continue;
				}
				if (prior.type === "slash" && prior.prev.type !== "bos" && rest[0] === "/") {
					const end = rest[1] !== void 0 ? "|$" : "";
					state.output = state.output.slice(0, -(prior.output + prev.output).length);
					prior.output = `(?:${prior.output}`;
					prev.type = "globstar";
					prev.output = `${globstar(opts)}${SLASH_LITERAL}|${SLASH_LITERAL}${end})`;
					prev.value += value;
					state.output += prior.output + prev.output;
					state.globstar = true;
					consume(value + advance());
					push({
						type: "slash",
						value: "/",
						output: ""
					});
					continue;
				}
				if (prior.type === "bos" && rest[0] === "/") {
					prev.type = "globstar";
					prev.value += value;
					prev.output = `(?:^|${SLASH_LITERAL}|${globstar(opts)}${SLASH_LITERAL})`;
					state.output = prev.output;
					state.globstar = true;
					consume(value + advance());
					push({
						type: "slash",
						value: "/",
						output: ""
					});
					continue;
				}
				state.output = state.output.slice(0, -prev.output.length);
				prev.type = "globstar";
				prev.output = globstar(opts);
				prev.value += value;
				state.output += prev.output;
				state.globstar = true;
				consume(value);
				continue;
			}
			const token = {
				type: "star",
				value,
				output: star
			};
			if (opts.bash === true) {
				token.output = ".*?";
				if (prev.type === "bos" || prev.type === "slash") token.output = nodot + token.output;
				push(token);
				continue;
			}
			if (prev && (prev.type === "bracket" || prev.type === "paren") && opts.regex === true) {
				token.output = value;
				push(token);
				continue;
			}
			if (state.index === state.start || prev.type === "slash" || prev.type === "dot") {
				if (prev.type === "dot") {
					state.output += NO_DOT_SLASH;
					prev.output += NO_DOT_SLASH;
				} else if (opts.dot === true) {
					state.output += NO_DOTS_SLASH;
					prev.output += NO_DOTS_SLASH;
				} else {
					state.output += nodot;
					prev.output += nodot;
				}
				if (peek() !== "*") {
					state.output += ONE_CHAR;
					prev.output += ONE_CHAR;
				}
			}
			push(token);
		}
		while (state.brackets > 0) {
			if (opts.strictBrackets === true) throw new SyntaxError(syntaxError("closing", "]"));
			state.output = utils.escapeLast(state.output, "[");
			decrement("brackets");
		}
		while (state.parens > 0) {
			if (opts.strictBrackets === true) throw new SyntaxError(syntaxError("closing", ")"));
			state.output = utils.escapeLast(state.output, "(");
			decrement("parens");
		}
		while (state.braces > 0) {
			if (opts.strictBrackets === true) throw new SyntaxError(syntaxError("closing", "}"));
			state.output = utils.escapeLast(state.output, "{");
			decrement("braces");
		}
		if (opts.strictSlashes !== true && (prev.type === "star" || prev.type === "bracket")) push({
			type: "maybe_slash",
			value: "",
			output: `${SLASH_LITERAL}?`
		});
		if (state.backtrack === true) {
			state.output = "";
			for (const token of state.tokens) {
				state.output += token.output != null ? token.output : token.value;
				if (token.suffix) state.output += token.suffix;
			}
		}
		return state;
	};
	/**
	* Fast paths for creating regular expressions for common glob patterns.
	* This can significantly speed up processing and has very little downside
	* impact when none of the fast paths match.
	*/
	parse.fastpaths = (input, options) => {
		const opts = { ...options };
		const max = typeof opts.maxLength === "number" ? Math.min(MAX_LENGTH, opts.maxLength) : MAX_LENGTH;
		const len = input.length;
		if (len > max) throw new SyntaxError(`Input length: ${len}, exceeds maximum allowed length: ${max}`);
		input = REPLACEMENTS[input] || input;
		const { DOT_LITERAL, SLASH_LITERAL, ONE_CHAR, DOTS_SLASH, NO_DOT, NO_DOTS, NO_DOTS_SLASH, STAR, START_ANCHOR } = constants.globChars(opts.windows);
		const nodot = opts.dot ? NO_DOTS : NO_DOT;
		const slashDot = opts.dot ? NO_DOTS_SLASH : NO_DOT;
		const capture = opts.capture ? "" : "?:";
		const state = {
			negated: false,
			prefix: ""
		};
		let star = opts.bash === true ? ".*?" : STAR;
		if (opts.capture) star = `(${star})`;
		const globstar = (opts) => {
			if (opts.noglobstar === true) return star;
			return `(${capture}(?:(?!${START_ANCHOR}${opts.dot ? DOTS_SLASH : DOT_LITERAL}).)*?)`;
		};
		const create = (str) => {
			switch (str) {
				case "*": return `${nodot}${ONE_CHAR}${star}`;
				case ".*": return `${DOT_LITERAL}${ONE_CHAR}${star}`;
				case "*.*": return `${nodot}${star}${DOT_LITERAL}${ONE_CHAR}${star}`;
				case "*/*": return `${nodot}${star}${SLASH_LITERAL}${ONE_CHAR}${slashDot}${star}`;
				case "**": return nodot + globstar(opts);
				case "**/*": return `(?:${nodot}${globstar(opts)}${SLASH_LITERAL})?${slashDot}${ONE_CHAR}${star}`;
				case "**/*.*": return `(?:${nodot}${globstar(opts)}${SLASH_LITERAL})?${slashDot}${star}${DOT_LITERAL}${ONE_CHAR}${star}`;
				case "**/.*": return `(?:${nodot}${globstar(opts)}${SLASH_LITERAL})?${DOT_LITERAL}${ONE_CHAR}${star}`;
				default: {
					const match = /^(.*?)\.(\w+)$/.exec(str);
					if (!match) return;
					const source = create(match[1]);
					if (!source) return;
					return source + DOT_LITERAL + match[2];
				}
			}
		};
		let source = create(utils.removePrefix(input, state));
		if (source && opts.strictSlashes !== true) source += `${SLASH_LITERAL}?`;
		return source;
	};
	module.exports = parse;
}));
//#endregion
//#region ../../node_modules/.pnpm/picomatch@4.0.7/node_modules/picomatch/lib/picomatch.js
var require_picomatch$1 = /* @__PURE__ */ __commonJSMin(((exports, module) => {
	var scan = require_scan();
	var parse = require_parse();
	var utils = require_utils();
	var constants = require_constants();
	var isObject = (val) => val && typeof val === "object" && !Array.isArray(val);
	/**
	* Creates a matcher function from one or more glob patterns. The
	* returned function takes a string to match as its first argument,
	* and returns true if the string is a match. The returned matcher
	* function also takes a boolean as the second argument that, when true,
	* returns an object with additional information.
	*
	* ```js
	* const picomatch = require('picomatch');
	* // picomatch(glob[, options]);
	*
	* const isMatch = picomatch('*.!(*a)');
	* console.log(isMatch('a.a')); //=> false
	* console.log(isMatch('a.b')); //=> true
	*
	* // For environments without `node.js`, `picomatch/posix` provides you a dependency-free matcher, without automatic OS detection.
	* const picomatch = require('picomatch/posix');
	* // the same API, defaulting to posix paths
	* const isMatch = picomatch('a/*');
	* console.log(isMatch('a\\b')); //=> false
	* console.log(isMatch('a/b')); //=> true
	*
	* // you can still configure the matcher function to accept windows paths
	* const isMatch = picomatch('a/*', { options: windows });
	* console.log(isMatch('a\\b')); //=> true
	* console.log(isMatch('a/b')); //=> true
	* ```
	* @name picomatch
	* @param {String|Array} `globs` One or more glob patterns.
	* @param {Object=} `options`
	* @return {Function=} Returns a matcher function.
	* @api public
	*/
	var picomatch = (glob, options, returnState = false) => {
		if (Array.isArray(glob)) {
			const fns = glob.map((input) => picomatch(input, options, returnState));
			const arrayMatcher = (str) => {
				for (const isMatch of fns) {
					const state = isMatch(str);
					if (state) return state;
				}
				return false;
			};
			return arrayMatcher;
		}
		const isState = isObject(glob) && glob.tokens && glob.input;
		if (glob === "" || typeof glob !== "string" && !isState) throw new TypeError("Expected pattern to be a non-empty string");
		const opts = options || {};
		const posix = opts.windows;
		const regex = isState ? picomatch.compileRe(glob, options) : picomatch.makeRe(glob, options, false, true);
		const state = regex.state;
		delete regex.state;
		let isIgnored = () => false;
		if (opts.ignore) {
			const ignoreOpts = {
				...options,
				ignore: null,
				onMatch: null,
				onResult: null
			};
			isIgnored = picomatch(opts.ignore, ignoreOpts, returnState);
		}
		const matcher = (input, returnObject = false) => {
			const { isMatch, match, output } = picomatch.test(input, regex, options, {
				glob,
				posix
			});
			const result = {
				glob,
				state,
				regex,
				posix,
				input,
				output,
				match,
				isMatch
			};
			if (typeof opts.onResult === "function") opts.onResult(result);
			if (isMatch === false) {
				result.isMatch = false;
				return returnObject ? result : false;
			}
			if (isIgnored(input)) {
				if (typeof opts.onIgnore === "function") opts.onIgnore(result);
				result.isMatch = false;
				return returnObject ? result : false;
			}
			if (typeof opts.onMatch === "function") opts.onMatch(result);
			return returnObject ? result : true;
		};
		if (returnState) matcher.state = state;
		return matcher;
	};
	/**
	* Test `input` with the given `regex`. This is used by the main
	* `picomatch()` function to test the input string.
	*
	* ```js
	* const picomatch = require('picomatch');
	* // picomatch.test(input, regex[, options]);
	*
	* console.log(picomatch.test('foo/bar', /^(?:([^/]*?)\/([^/]*?))$/));
	* // { isMatch: true, match: [ 'foo/', 'foo', 'bar' ], output: 'foo/bar' }
	* ```
	* @param {String} `input` String to test.
	* @param {RegExp} `regex`
	* @return {Object} Returns an object with matching info.
	* @api public
	*/
	picomatch.test = (input, regex, options, { glob, posix } = {}) => {
		if (typeof input !== "string") throw new TypeError("Expected input to be a string");
		if (input === "") return {
			isMatch: false,
			output: ""
		};
		const opts = options || {};
		const format = opts.format || (posix ? utils.toPosixSlashes : null);
		let match = input === glob;
		let output = match && format ? format(input) : input;
		if (match === false) {
			output = format ? format(input) : input;
			match = output === glob;
		}
		if (match === false || opts.capture === true) {
			if (opts.matchBase === true || opts.basename === true) match = picomatch.matchBase(input, regex, options, posix);
			else match = regex.exec(output);
		}
		return {
			isMatch: Boolean(match),
			match,
			output
		};
	};
	/**
	* Match the basename of a filepath.
	*
	* ```js
	* const picomatch = require('picomatch');
	* // picomatch.matchBase(input, glob[, options]);
	* console.log(picomatch.matchBase('foo/bar.js', '*.js'); // true
	* ```
	* @param {String} `input` String to test.
	* @param {RegExp|String} `glob` Glob pattern or regex created by [.makeRe](#makeRe).
	* @return {Boolean}
	* @api public
	*/
	picomatch.matchBase = (input, glob, options, posix = options && options.windows) => {
		return (glob instanceof RegExp ? glob : picomatch.makeRe(glob, options)).test(utils.basename(input, { windows: posix }));
	};
	/**
	* Returns true if **any** of the given glob `patterns` match the specified `string`.
	*
	* ```js
	* const picomatch = require('picomatch');
	* // picomatch.isMatch(string, patterns[, options]);
	*
	* console.log(picomatch.isMatch('a.a', ['b.*', '*.a'])); //=> true
	* console.log(picomatch.isMatch('a.a', 'b.*')); //=> false
	* ```
	* @param {String|Array} str The string to test.
	* @param {String|Array} patterns One or more glob patterns to use for matching.
	* @param {Object} [options] See available [options](#options).
	* @return {Boolean} Returns true if any patterns match `str`
	* @api public
	*/
	picomatch.isMatch = (str, patterns, options) => picomatch(patterns, options)(str);
	/**
	* Parse a glob pattern to create the source string for a regular
	* expression.
	*
	* ```js
	* const picomatch = require('picomatch');
	* const result = picomatch.parse(pattern[, options]);
	* ```
	* @param {String} `pattern`
	* @param {Object} `options`
	* @return {Object} Returns an object with useful properties and output to be used as a regex source string.
	* @api public
	*/
	picomatch.parse = (pattern, options) => {
		if (Array.isArray(pattern)) return pattern.map((p) => picomatch.parse(p, options));
		return parse(pattern, {
			...options,
			fastpaths: false
		});
	};
	/**
	* Scan a glob pattern to separate the pattern into segments.
	*
	* ```js
	* const picomatch = require('picomatch');
	* // picomatch.scan(input[, options]);
	*
	* const result = picomatch.scan('!./foo/*.js');
	* console.log(result);
	* { prefix: '!./',
	*   input: '!./foo/*.js',
	*   start: 3,
	*   base: 'foo',
	*   glob: '*.js',
	*   isBrace: false,
	*   isBracket: false,
	*   isGlob: true,
	*   isExtglob: false,
	*   isGlobstar: false,
	*   negated: true }
	* ```
	* @param {String} `input` Glob pattern to scan.
	* @param {Object} `options`
	* @return {Object} Returns an object with
	* @api public
	*/
	picomatch.scan = (input, options) => scan(input, options);
	/**
	* Compile a regular expression from the `state` object returned by the
	* [parse()](#parse) method.
	*
	* ```js
	* const picomatch = require('picomatch');
	* const state = picomatch.parse('*.js');
	* // picomatch.compileRe(state[, options]);
	*
	* console.log(picomatch.compileRe(state));
	* //=> /^(?:(?!\.)(?=.)[^/]*?\.js)$/
	* ```
	* @param {Object} `state`
	* @param {Object} `options`
	* @param {Boolean} `returnOutput` Intended for implementors, this argument allows you to return the raw output from the parser.
	* @param {Boolean} `returnState` Adds the state to a `state` property on the returned regex. Useful for implementors and debugging.
	* @return {RegExp}
	* @api public
	*/
	picomatch.compileRe = (state, options, returnOutput = false, returnState = false) => {
		if (returnOutput === true) return state.output;
		const opts = options || {};
		const prepend = opts.contains ? "" : "^";
		const append = opts.contains ? "" : "$";
		let source = `${prepend}(?:${state.output})${append}`;
		if (state && state.negated === true) source = `^(?!${source}).*$`;
		const regex = picomatch.toRegex(source, options);
		if (returnState === true) regex.state = state;
		return regex;
	};
	/**
	* Create a regular expression from a parsed glob pattern.
	*
	* ```js
	* const picomatch = require('picomatch');
	* // picomatch.makeRe(state[, options]);
	*
	* const result = picomatch.makeRe('*.js');
	* console.log(result);
	* //=> /^(?:(?!\.)(?=.)[^/]*?\.js)$/
	* ```
	* @param {String} `state` The object returned from the `.parse` method.
	* @param {Object} `options`
	* @param {Boolean} `returnOutput` Implementors may use this argument to return the compiled output, instead of a regular expression. This is not exposed on the options to prevent end-users from mutating the result.
	* @param {Boolean} `returnState` Implementors may use this argument to return the state from the parsed glob with the returned regular expression.
	* @return {RegExp} Returns a regex created from the given pattern.
	* @api public
	*/
	picomatch.makeRe = (input, options = {}, returnOutput = false, returnState = false) => {
		if (!input || typeof input !== "string") throw new TypeError("Expected a non-empty string");
		let parsed = {
			negated: false,
			fastpaths: true
		};
		if (options.fastpaths !== false && (input[0] === "." || input[0] === "*")) parsed.output = parse.fastpaths(input, options);
		if (!parsed.output) parsed = parse(input, options);
		return picomatch.compileRe(parsed, options, returnOutput, returnState);
	};
	/**
	* Create a regular expression from the given regex source string.
	*
	* ```js
	* const picomatch = require('picomatch');
	* // picomatch.toRegex(source[, options]);
	*
	* const { output } = picomatch.parse('*.js');
	* console.log(picomatch.toRegex(output));
	* //=> /^(?:(?!\.)(?=.)[^/]*?\.js)$/
	* ```
	* @param {String} `source` Regular expression source string.
	* @param {Object} `options`
	* @return {RegExp}
	* @api public
	*/
	picomatch.toRegex = (source, options) => {
		try {
			const opts = options || {};
			return new RegExp(source, opts.flags || (opts.nocase ? "i" : ""));
		} catch (err) {
			if (options && options.debug === true) throw err;
			return /$^/;
		}
	};
	/**
	* Picomatch constants.
	* @return {Object}
	*/
	picomatch.constants = constants;
	/**
	* Expose "picomatch"
	*/
	module.exports = picomatch;
}));
//#endregion
//#region ../../node_modules/.pnpm/@valibot+to-json-schema@1.8.0_valibot@1.5.0_typescript@6.0.3_/node_modules/@valibot/to-json-schema/dist/index.mjs
var import_picomatch = /* @__PURE__ */ __toESM((/* @__PURE__ */ __commonJSMin(((exports, module) => {
	var pico = require_picomatch$1();
	var utils = require_utils();
	function picomatch(glob, options, returnState = false) {
		if (options && (options.windows === null || options.windows === void 0)) options = {
			...options,
			windows: utils.isWindows()
		};
		return pico(glob, options, returnState);
	}
	Object.assign(picomatch, pico);
	module.exports = picomatch;
})))(), 1);
/**
* Adds an error message to the errors array.
*
* @param errors The array of error messages.
* @param message The error message to add.
*
* @returns The new errors.
*/
function addError(errors, message) {
	if (errors) {
		errors.push(message);
		return errors;
	}
	return [message];
}
var ESCAPE_REGEX = /[.*+?^${}()|[\]\\]/g;
/**
* Escapes special regex characters in a string.
*
* @param string The string to escape.
*
* @returns The escaped string.
*/
function escapeRegExp(string) {
	return string.replace(ESCAPE_REGEX, "\\$&");
}
/**
* Throws an error or logs a warning based on the configuration.
*
* @param message The message to throw or log.
* @param config The conversion configuration.
*/
function handleError(message, config) {
	switch (config?.errorMode) {
		case "ignore": break;
		case "warn":
			console.warn(message);
			break;
		default: throw new Error(message);
	}
}
/**
* Whether a value is JSON compatible for a const keyword.
*
* @param value The value to check.
*
* @returns Whether the value is JSON compatible.
*/
function isJsonConstValue(value) {
	return typeof value === "boolean" || typeof value === "number" && Number.isFinite(value) || typeof value === "string";
}
/**
* Whether values are JSON compatible for an enum keyword.
*
* @param values The values to check.
*
* @returns Whether the values are JSON compatible.
*/
function isJsonEnumValues(values) {
	return values.every(isJsonConstValue);
}
/**
* Schema reference map with collision-free reference ID allocation.
*/
var ReferenceMap = class extends Map {
	/**
	* Creates an empty schema reference map.
	*/
	constructor() {
		super();
		this.count = 0;
		this.usedIds = /* @__PURE__ */ new Set();
	}
	/**
	* Adds a schema reference and reserves its ID.
	*
	* @param schema The Valibot schema.
	* @param referenceId The reference ID.
	*
	* @returns The reference map.
	*/
	set(schema, referenceId) {
		this.usedIds.add(referenceId);
		return super.set(schema, referenceId);
	}
	/**
	* Creates a reference ID that is not yet used.
	*
	* @param definitions The JSON Schema definitions.
	*
	* @returns The unused reference ID.
	*/
	createId(definitions) {
		while (this.usedIds.has(`${this.count}`) || `${this.count}` in definitions) this.count++;
		return `${this.count++}`;
	}
};
/**
* Returns the stricter lower bound.
*
* @param current The current lower bound.
* @param value The new lower bound.
*
* @returns The stricter lower bound.
*/
function getLowerBound(current, value) {
	if (typeof current !== "number" || !Number.isFinite(current) || value > current) return value;
	return current;
}
/**
* Returns the stricter upper bound.
*
* @param current The current upper bound.
* @param value The new upper bound.
*
* @returns The stricter upper bound.
*/
function getUpperBound(current, value) {
	if (typeof current !== "number" || !Number.isFinite(current) || value < current) return value;
	return current;
}
/**
* Returns the combined not restriction.
*
* @param current The current not restriction.
* @param value The new not restriction.
*
* @returns The combined not restriction.
*/
/* @__NO_SIDE_EFFECTS__ */
function getNotRestriction(current, value) {
	return current !== void 0 ? { anyOf: [current, value] } : value;
}
/**
* Intersects allowed values with an existing enum restriction.
*
* @param jsonSchema The JSON Schema object.
* @param values The allowed values.
*/
function intersectEnum(jsonSchema, values) {
	let enumValues = jsonSchema.enum ?? values;
	if (jsonSchema.enum) {
		const valueSet = new Set(values);
		enumValues = enumValues.filter((value) => valueSet.has(value));
	}
	if (enumValues.length) jsonSchema.enum = [...new Set(enumValues)];
	else {
		delete jsonSchema.enum;
		jsonSchema.not = /* @__PURE__ */ getNotRestriction(jsonSchema.not, {});
	}
}
/**
* Converts any supported Valibot action to the JSON Schema format.
*
* @param jsonSchema The JSON Schema object.
* @param valibotAction The Valibot action object.
* @param config The conversion configuration.
*
* @returns The converted JSON Schema.
*/
function convertAction(jsonSchema, valibotAction, config) {
	if (config?.ignoreActions?.includes(valibotAction.type)) return jsonSchema;
	let errors;
	switch (valibotAction.type) {
		case "base64":
			jsonSchema.contentEncoding = "base64";
			break;
		case "bic":
		case "cuid2":
		case "decimal":
		case "digits":
		case "domain":
		case "emoji":
		case "hash":
		case "hexadecimal":
		case "hex_color":
		case "isrc":
		case "iso_time_second":
		case "iso_week":
		case "ksuid":
		case "mac":
		case "mac48":
		case "mac64":
		case "nanoid":
		case "octal":
		case "slug":
		case "ulid":
			if (jsonSchema.pattern) errors = addError(errors, `The "${valibotAction.type}" action is not supported in combination with another regex action.`);
			else jsonSchema.pattern = valibotAction.requirement.source;
			break;
		case "description":
			jsonSchema.description = valibotAction.description;
			break;
		case "email":
		case "rfc_email":
			jsonSchema.format = "email";
			break;
		case "ends_with":
			if (jsonSchema.pattern) errors = addError(errors, `The "${valibotAction.type}" action is not supported in combination with another regex action.`);
			else jsonSchema.pattern = `${escapeRegExp(valibotAction.requirement)}$`;
			break;
		case "empty":
			if (jsonSchema.type === "array") jsonSchema.maxItems = 0;
			else {
				if (jsonSchema.type !== "string") errors = addError(errors, `The "${valibotAction.type}" action is not supported on type "${jsonSchema.type}".`);
				jsonSchema.maxLength = 0;
			}
			break;
		case "entries":
			if (!Number.isInteger(valibotAction.requirement) || valibotAction.requirement < 0) {
				errors = addError(errors, "The requirement of the \"entries\" action must be a non-negative integer.");
				break;
			}
			jsonSchema.minProperties = getLowerBound(jsonSchema.minProperties, valibotAction.requirement);
			jsonSchema.maxProperties = getUpperBound(jsonSchema.maxProperties, valibotAction.requirement);
			break;
		case "examples":
			if (Array.isArray(jsonSchema.examples)) jsonSchema.examples = [...jsonSchema.examples, ...valibotAction.examples];
			else jsonSchema.examples = valibotAction.examples;
			break;
		case "gt_value":
			if (jsonSchema.type !== "number" && jsonSchema.type !== "integer") {
				errors = addError(errors, `The "gt_value" action is not supported on type "${jsonSchema.type}".`);
				break;
			}
			if (!Number.isFinite(valibotAction.requirement)) {
				errors = addError(errors, "The requirement of the \"gt_value\" action is not JSON compatible.");
				break;
			}
			if (config?.target === "openapi-3.0") {
				errors = addError(errors, "The \"gt_value\" action is not supported for OpenAPI 3.0.");
				break;
			}
			jsonSchema.exclusiveMinimum = getLowerBound(jsonSchema.exclusiveMinimum, valibotAction.requirement);
			break;
		case "includes":
			if (jsonSchema.pattern) errors = addError(errors, `The "${valibotAction.type}" action is not supported in combination with another regex action.`);
			else jsonSchema.pattern = escapeRegExp(valibotAction.requirement);
			break;
		case "integer":
			jsonSchema.type = "integer";
			break;
		case "ipv4":
			jsonSchema.format = "ipv4";
			break;
		case "ipv6":
			jsonSchema.format = "ipv6";
			break;
		case "iso_date":
			jsonSchema.format = "date";
			break;
		case "iso_date_time":
		case "iso_timestamp":
			jsonSchema.format = "date-time";
			break;
		case "iso_time":
			jsonSchema.format = "time";
			break;
		case "jws_compact":
			if (jsonSchema.pattern) errors = addError(errors, `The "${valibotAction.type}" action is not supported in combination with another regex action.`);
			else jsonSchema.pattern = valibotAction.requirement.source;
			break;
		case "length":
			if (!Number.isInteger(valibotAction.requirement) || valibotAction.requirement < 0) {
				errors = addError(errors, "The requirement of the \"length\" action must be a non-negative integer.");
				break;
			}
			if (jsonSchema.type === "array") {
				jsonSchema.minItems = getLowerBound(jsonSchema.minItems, valibotAction.requirement);
				jsonSchema.maxItems = getUpperBound(jsonSchema.maxItems, valibotAction.requirement);
			} else {
				if (jsonSchema.type !== "string") errors = addError(errors, `The "${valibotAction.type}" action is not supported on type "${jsonSchema.type}".`);
				jsonSchema.minLength = getLowerBound(jsonSchema.minLength, valibotAction.requirement);
				jsonSchema.maxLength = getUpperBound(jsonSchema.maxLength, valibotAction.requirement);
			}
			break;
		case "lt_value":
			if (jsonSchema.type !== "number" && jsonSchema.type !== "integer") {
				errors = addError(errors, `The "lt_value" action is not supported on type "${jsonSchema.type}".`);
				break;
			}
			if (!Number.isFinite(valibotAction.requirement)) {
				errors = addError(errors, "The requirement of the \"lt_value\" action is not JSON compatible.");
				break;
			}
			if (config?.target === "openapi-3.0") {
				errors = addError(errors, "The \"lt_value\" action is not supported for OpenAPI 3.0.");
				break;
			}
			jsonSchema.exclusiveMaximum = getUpperBound(jsonSchema.exclusiveMaximum, valibotAction.requirement);
			break;
		case "max_entries":
			if (!Number.isInteger(valibotAction.requirement) || valibotAction.requirement < 0) {
				errors = addError(errors, "The requirement of the \"max_entries\" action must be a non-negative integer.");
				break;
			}
			jsonSchema.maxProperties = getUpperBound(jsonSchema.maxProperties, valibotAction.requirement);
			break;
		case "max_length":
			if (!Number.isInteger(valibotAction.requirement) || valibotAction.requirement < 0) {
				errors = addError(errors, "The requirement of the \"max_length\" action must be a non-negative integer.");
				break;
			}
			if (jsonSchema.type === "array") jsonSchema.maxItems = getUpperBound(jsonSchema.maxItems, valibotAction.requirement);
			else {
				if (jsonSchema.type !== "string") errors = addError(errors, `The "${valibotAction.type}" action is not supported on type "${jsonSchema.type}".`);
				jsonSchema.maxLength = getUpperBound(jsonSchema.maxLength, valibotAction.requirement);
			}
			break;
		case "max_value":
			if (jsonSchema.type !== "number" && jsonSchema.type !== "integer") {
				errors = addError(errors, `The "max_value" action is not supported on type "${jsonSchema.type}".`);
				break;
			}
			if (!Number.isFinite(valibotAction.requirement)) {
				errors = addError(errors, "The requirement of the \"max_value\" action is not JSON compatible.");
				break;
			}
			jsonSchema.maximum = getUpperBound(jsonSchema.maximum, valibotAction.requirement);
			break;
		case "metadata":
			if (typeof valibotAction.metadata.title === "string") jsonSchema.title = valibotAction.metadata.title;
			if (typeof valibotAction.metadata.description === "string") jsonSchema.description = valibotAction.metadata.description;
			if (Array.isArray(valibotAction.metadata.examples)) if (Array.isArray(jsonSchema.examples)) jsonSchema.examples = [...jsonSchema.examples, ...valibotAction.metadata.examples];
			else jsonSchema.examples = valibotAction.metadata.examples;
			for (const key of Object.keys(valibotAction.metadata)) if (key !== "title" && key !== "description" && key !== "examples" && key !== "__proto__") jsonSchema[key] = valibotAction.metadata[key];
			break;
		case "min_entries":
			if (!Number.isInteger(valibotAction.requirement) || valibotAction.requirement < 0) {
				errors = addError(errors, "The requirement of the \"min_entries\" action must be a non-negative integer.");
				break;
			}
			jsonSchema.minProperties = getLowerBound(jsonSchema.minProperties, valibotAction.requirement);
			break;
		case "min_length":
			if (!Number.isInteger(valibotAction.requirement) || valibotAction.requirement < 0) {
				errors = addError(errors, "The requirement of the \"min_length\" action must be a non-negative integer.");
				break;
			}
			if (jsonSchema.type === "array") jsonSchema.minItems = getLowerBound(jsonSchema.minItems, valibotAction.requirement);
			else {
				if (jsonSchema.type !== "string") errors = addError(errors, `The "${valibotAction.type}" action is not supported on type "${jsonSchema.type}".`);
				jsonSchema.minLength = getLowerBound(jsonSchema.minLength, valibotAction.requirement);
			}
			break;
		case "min_value":
			if (jsonSchema.type !== "number" && jsonSchema.type !== "integer") {
				errors = addError(errors, `The "min_value" action is not supported on type "${jsonSchema.type}".`);
				break;
			}
			if (!Number.isFinite(valibotAction.requirement)) {
				errors = addError(errors, "The requirement of the \"min_value\" action is not JSON compatible.");
				break;
			}
			jsonSchema.minimum = getLowerBound(jsonSchema.minimum, valibotAction.requirement);
			break;
		case "multiple_of":
			jsonSchema.multipleOf = valibotAction.requirement;
			break;
		case "non_empty":
			if (jsonSchema.type === "array") jsonSchema.minItems = getLowerBound(jsonSchema.minItems, 1);
			else {
				if (jsonSchema.type !== "string") errors = addError(errors, `The "${valibotAction.type}" action is not supported on type "${jsonSchema.type}".`);
				jsonSchema.minLength = getLowerBound(jsonSchema.minLength, 1);
			}
			break;
		case "not_value":
			if (!isJsonConstValue(valibotAction.requirement)) {
				errors = addError(errors, "The requirement of the \"not_value\" action is not JSON compatible.");
				break;
			}
			jsonSchema.not = /* @__PURE__ */ getNotRestriction(jsonSchema.not, config?.target === "openapi-3.0" ? { enum: [valibotAction.requirement] } : { const: valibotAction.requirement });
			break;
		case "not_values":
			if (!isJsonEnumValues(valibotAction.requirement)) {
				errors = addError(errors, "A requirement of the \"not_values\" action is not JSON compatible.");
				break;
			}
			if (valibotAction.requirement.length) jsonSchema.not = /* @__PURE__ */ getNotRestriction(jsonSchema.not, { enum: [...new Set(valibotAction.requirement)] });
			break;
		case "regex":
			if (valibotAction.requirement.flags) errors = addError(errors, "RegExp flags are not supported by JSON Schema.");
			if (jsonSchema.pattern) errors = addError(errors, `The "${valibotAction.type}" action is not supported in combination with another regex action.`);
			else jsonSchema.pattern = valibotAction.requirement.source;
			break;
		case "safe_integer":
			jsonSchema.type = "integer";
			jsonSchema.minimum = getLowerBound(jsonSchema.minimum, Number.MIN_SAFE_INTEGER);
			jsonSchema.maximum = getUpperBound(jsonSchema.maximum, Number.MAX_SAFE_INTEGER);
			break;
		case "starts_with":
			if (jsonSchema.pattern) errors = addError(errors, `The "${valibotAction.type}" action is not supported in combination with another regex action.`);
			else jsonSchema.pattern = `^${escapeRegExp(valibotAction.requirement)}`;
			break;
		case "title":
			jsonSchema.title = valibotAction.title;
			break;
		case "url":
			jsonSchema.format = "uri";
			break;
		case "uuid":
			jsonSchema.format = "uuid";
			break;
		case "value":
			if (!isJsonConstValue(valibotAction.requirement)) {
				errors = addError(errors, "The requirement of the \"value\" action is not JSON compatible.");
				break;
			}
			if (config?.target === "openapi-3.0") intersectEnum(jsonSchema, [valibotAction.requirement]);
			else if ("const" in jsonSchema && jsonSchema.const !== valibotAction.requirement) errors = addError(errors, `The "${valibotAction.type}" action is not supported in combination with a different "const" restriction.`);
			else jsonSchema.const = valibotAction.requirement;
			break;
		case "values":
			if (!isJsonEnumValues(valibotAction.requirement)) {
				errors = addError(errors, "A requirement of the \"values\" action is not JSON compatible.");
				break;
			}
			intersectEnum(jsonSchema, valibotAction.requirement);
			break;
		default: errors = addError(errors, `The "${valibotAction.type}" action cannot be converted to JSON Schema.`);
	}
	if (config?.overrideAction) {
		const actionOverride = config.overrideAction({
			valibotAction,
			jsonSchema,
			errors
		});
		if (actionOverride) return { ...actionOverride };
	}
	if (errors) for (const message of errors) handleError(message, config);
	return jsonSchema;
}
/**
* Flattens a Valibot pipe by recursively expanding nested pipes.
*
* @param pipe The pipeline to flatten.
*
* @returns A flat pipeline.
*/
function flattenPipe(pipe) {
	return pipe.flatMap((item) => "pipe" in item ? flattenPipe(item.pipe) : item);
}
/**
* Returns the JSON Pointer reference for a definition key.
*
* @param referenceId The unescaped definition key.
*
* @returns The encoded JSON Pointer fragment.
*/
function getDefinitionRef(referenceId) {
	return `#/$defs/${referenceId.replaceAll("~", "~0").replaceAll("/", "~1")}`;
}
/**
* Converts any supported Valibot schema to the JSON Schema format.
*
* @param jsonSchema The JSON Schema object.
* @param valibotSchema The Valibot schema object.
* @param config The conversion configuration.
* @param context The conversion context.
* @param skipRef Whether to skip using a reference.
*
* @returns The converted JSON Schema.
*/
function convertSchema(jsonSchema, valibotSchema, config, context, skipRef = false) {
	if (!skipRef) {
		const referenceId = context.referenceMap.get(valibotSchema);
		if (referenceId) {
			jsonSchema.$ref = getDefinitionRef(referenceId);
			if (config?.overrideRef) {
				const refOverride = config.overrideRef({
					...context,
					referenceId,
					valibotSchema,
					jsonSchema
				});
				if (refOverride) jsonSchema.$ref = refOverride;
			}
			return jsonSchema;
		}
	}
	if ("pipe" in valibotSchema) {
		const flatPipe = flattenPipe(valibotSchema.pipe);
		let startIndex = 0;
		let stopIndex = flatPipe.length - 1;
		if (config?.typeMode === "input") {
			const inputStopIndex = flatPipe.slice(1).findIndex((item) => item.kind === "schema" || item.kind === "transformation" && (item.type === "find_item" || item.type === "parse_json" || item.type === "raw_transform" || item.type === "reduce_items" || item.type === "stringify_json" || item.type === "to_bigint" || item.type === "to_boolean" || item.type === "to_date" || item.type === "to_number" || item.type === "to_string" || item.type === "transform"));
			if (inputStopIndex !== -1) stopIndex = inputStopIndex;
		} else if (config?.typeMode === "output") {
			const outputStartIndex = flatPipe.findLastIndex((item) => item.kind === "schema");
			if (outputStartIndex !== -1) startIndex = outputStartIndex;
		}
		for (let index = startIndex; index <= stopIndex; index++) {
			const valibotPipeItem = flatPipe[index];
			if (valibotPipeItem.kind === "schema") {
				if (index > startIndex) handleError("Set the \"typeMode\" config to \"input\" or \"output\" to convert pipelines with multiple schemas.", config);
				jsonSchema = convertSchema(jsonSchema, valibotPipeItem, config, context, true);
			} else jsonSchema = convertAction(jsonSchema, valibotPipeItem, config);
		}
		return jsonSchema;
	}
	let errors;
	switch (valibotSchema.type) {
		case "boolean":
			jsonSchema.type = "boolean";
			break;
		case "null":
			if (config?.target === "openapi-3.0") jsonSchema.enum = [null];
			else jsonSchema.type = "null";
			break;
		case "number":
			jsonSchema.type = "number";
			break;
		case "string":
			jsonSchema.type = "string";
			break;
		case "array":
			jsonSchema.type = "array";
			jsonSchema.items = convertSchema({}, valibotSchema.item, config, context);
			break;
		case "tuple":
		case "tuple_with_rest":
		case "loose_tuple":
		case "strict_tuple":
			jsonSchema.type = "array";
			if (config?.target === "openapi-3.0") {
				jsonSchema.items = { anyOf: [] };
				jsonSchema.minItems = valibotSchema.items.length;
				for (const item of valibotSchema.items) jsonSchema.items.anyOf.push(convertSchema({}, item, config, context));
				if (valibotSchema.type === "tuple_with_rest") jsonSchema.items.anyOf.push(convertSchema({}, valibotSchema.rest, config, context));
				else if (valibotSchema.type === "strict_tuple" || valibotSchema.type === "tuple") jsonSchema.maxItems = valibotSchema.items.length;
			} else if (config?.target === "draft-2020-12") {
				jsonSchema.prefixItems = [];
				jsonSchema.minItems = valibotSchema.items.length;
				for (const item of valibotSchema.items) jsonSchema.prefixItems.push(convertSchema({}, item, config, context));
				if (valibotSchema.type === "tuple_with_rest") jsonSchema.items = convertSchema({}, valibotSchema.rest, config, context);
				else if (valibotSchema.type === "strict_tuple") jsonSchema.items = false;
			} else {
				jsonSchema.items = [];
				jsonSchema.minItems = valibotSchema.items.length;
				for (const item of valibotSchema.items) jsonSchema.items.push(convertSchema({}, item, config, context));
				if (valibotSchema.type === "tuple_with_rest") jsonSchema.additionalItems = convertSchema({}, valibotSchema.rest, config, context);
				else if (valibotSchema.type === "strict_tuple") jsonSchema.additionalItems = false;
			}
			break;
		case "object":
		case "object_with_rest":
		case "loose_object":
		case "strict_object":
			jsonSchema.type = "object";
			jsonSchema.properties = {};
			jsonSchema.required = [];
			for (const key in valibotSchema.entries) {
				const entry = valibotSchema.entries[key];
				jsonSchema.properties[key] = convertSchema({}, entry, config, context);
				if (entry.type !== "exact_optional" && entry.type !== "nullish" && entry.type !== "optional") jsonSchema.required.push(key);
			}
			if (valibotSchema.type === "object_with_rest") jsonSchema.additionalProperties = convertSchema({}, valibotSchema.rest, config, context);
			else if (valibotSchema.type === "strict_object") jsonSchema.additionalProperties = false;
			break;
		case "record":
			if (config?.target === "openapi-3.0" && "pipe" in valibotSchema.key) errors = addError(errors, "The \"record\" schema with a schema for the key that contains a \"pipe\" cannot be converted to JSON Schema.");
			if (valibotSchema.key.type !== "string") errors = addError(errors, `The "record" schema with the "${valibotSchema.key.type}" schema for the key cannot be converted to JSON Schema.`);
			jsonSchema.type = "object";
			if (config?.target !== "openapi-3.0") jsonSchema.propertyNames = convertSchema({}, valibotSchema.key, config, context);
			jsonSchema.additionalProperties = convertSchema({}, valibotSchema.value, config, context);
			break;
		case "any":
		case "unknown": break;
		case "never":
			jsonSchema.not = {};
			break;
		case "nullable":
		case "nullish":
			if (config?.target === "openapi-3.0") {
				const innerSchema = convertSchema({}, valibotSchema.wrapped, config, context);
				Object.assign(jsonSchema, innerSchema);
				jsonSchema.nullable = true;
			} else jsonSchema.anyOf = [convertSchema({}, valibotSchema.wrapped, config, context), { type: "null" }];
			if (valibotSchema.default !== void 0) jsonSchema.default = typeof valibotSchema.default === "function" ? valibotSchema.default() : valibotSchema.default;
			break;
		case "exact_optional":
		case "optional":
		case "undefinedable":
			jsonSchema = convertSchema(jsonSchema, valibotSchema.wrapped, config, context);
			if (valibotSchema.default !== void 0) jsonSchema.default = typeof valibotSchema.default === "function" ? valibotSchema.default() : valibotSchema.default;
			break;
		case "literal":
			if (!isJsonConstValue(valibotSchema.literal)) {
				errors = addError(errors, "The value of the \"literal\" schema is not JSON compatible.");
				break;
			}
			if (config?.target === "openapi-3.0") jsonSchema.enum = [valibotSchema.literal];
			else jsonSchema.const = valibotSchema.literal;
			break;
		case "enum":
		case "picklist": {
			const options = valibotSchema.options;
			if (!options.every((option) => typeof option === "string" || typeof option === "number" && Number.isFinite(option))) {
				errors = addError(errors, `An option of the "${valibotSchema.type}" schema is not JSON compatible.`);
				break;
			}
			jsonSchema.enum = options;
			if (options.every((option) => typeof option === "string")) jsonSchema.type = "string";
			else if (options.every((option) => typeof option === "number")) jsonSchema.type = "number";
			else if (config?.target !== "openapi-3.0") jsonSchema.type = ["string", "number"];
			break;
		}
		case "union":
			jsonSchema.anyOf = valibotSchema.options.map((option) => convertSchema({}, option, config, context));
			break;
		case "variant":
			jsonSchema.oneOf = valibotSchema.options.map((option) => convertSchema({}, option, config, context));
			break;
		case "intersect":
			jsonSchema.allOf = valibotSchema.options.map((option) => convertSchema({}, option, config, context));
			break;
		case "lazy": {
			let wrappedValibotSchema = context.getterMap.get(valibotSchema.getter);
			if (!wrappedValibotSchema) {
				wrappedValibotSchema = valibotSchema.getter(void 0);
				context.getterMap.set(valibotSchema.getter, wrappedValibotSchema);
			}
			let referenceId = context.referenceMap.get(wrappedValibotSchema);
			if (!referenceId) {
				referenceId = context.referenceMap.createId(context.definitions);
				context.referenceMap.set(wrappedValibotSchema, referenceId);
				context.definitions[referenceId] = convertSchema({}, wrappedValibotSchema, config, context, true);
			}
			jsonSchema.$ref = getDefinitionRef(referenceId);
			if (config?.overrideRef) {
				const refOverride = config.overrideRef({
					...context,
					referenceId,
					valibotSchema: wrappedValibotSchema,
					jsonSchema
				});
				if (refOverride) jsonSchema.$ref = refOverride;
			}
			break;
		}
		default: errors = addError(errors, `The "${valibotSchema.type}" schema cannot be converted to JSON Schema.`);
	}
	if (config?.overrideSchema) {
		const schemaOverride = config.overrideSchema({
			...context,
			referenceId: context.referenceMap.get(valibotSchema),
			valibotSchema,
			jsonSchema,
			errors
		});
		if (schemaOverride) return { ...schemaOverride };
	}
	if (errors) for (const message of errors) handleError(message, config);
	return jsonSchema;
}
/**
* Converts a Valibot schema to the JSON Schema format.
*
* @param schema The Valibot schema object.
* @param config The JSON Schema configuration.
*
* @returns The converted JSON Schema.
*/
function toJsonSchema(schema, config) {
	const context = {
		definitions: {},
		referenceMap: new ReferenceMap(),
		getterMap: /* @__PURE__ */ new Map()
	};
	const definitions = config?.definitions ?? void 0;
	if (definitions) {
		for (const key in definitions) context.referenceMap.set(definitions[key], key);
		for (const key in definitions) context.definitions[key] = convertSchema({}, definitions[key], config, context, true);
	}
	const jsonSchema = convertSchema({}, schema, config, context);
	const target = config?.target ?? "draft-07";
	if (target === "draft-2020-12") jsonSchema.$schema = "https://json-schema.org/draft/2020-12/schema";
	else if (target === "draft-07") jsonSchema.$schema = "http://json-schema.org/draft-07/schema#";
	if (context.referenceMap.size) jsonSchema.$defs = context.definitions;
	return jsonSchema;
}
//#endregion
//#region ../../node_modules/.pnpm/@pulls.review+core@0.3.1_typescript@6.0.3_ws@8.22.0_zod@4.6.5/node_modules/@pulls.review/core/dist/llm.js
var isGeneratedPath = (0, import_picomatch.default)(GENERATED_PATTERNS);
/** Renders a set of files as compact diff text for a prompt - the model-facing view of a patch. */
function renderFilesAsText(files) {
	return files.map((file) => {
		const rename = file.previousPath ? ` (renamed from ${file.previousPath})` : "";
		const header = `### ${file.path}${rename} [${file.status}, +${file.additions}/-${file.deletions}]`;
		if (file.isBinary) return `${header}\n(binary file, no diff shown)`;
		if (file.truncated) return `${header}\n(diff too large, omitted)`;
		if (isGeneratedPath(file.path)) return `${header}\n(generated file, diff omitted to save tokens)`;
		return `${header}\n${file.hunks.map((hunk) => `${hunk.header}\n${hunk.patch}`).join("\n")}`;
	}).join("\n\n");
}
var ROLE_SECTION = `<role>
You organize a GitHub pull request's changed files into review groups, so a reviewer can read the PR feature by feature instead of file by file.
</role>

<grouping_principles>
- Group by intent, not by directory. One feature touching several modules is ONE group.
- Keep groups flat. Use "children" only when a group is too large to read in one pass (more than 15 files) and splits into clearly distinct sub-areas. Never create a single-file child or a group with only one child.
- Use the fewest groups that still separate independent intents. A typical PR has 1-5 groups.
- Tests, stories and fixtures usually go in a group of their own, separate from the code they cover.
- Order groups by review priority: the core change first, supporting changes next, mechanical changes (lockfiles, generated files, formatting) last.
- Every path in the manifest goes into exactly one group or child.
- Commit messages, when listed, hint at the author's intents. Group by the final change, not by commit: fixup and WIP commits often mix concerns.
</grouping_principles>`;
var OUTPUT_SECTION = `<output>
- "summary" and "overallSummary" explain why over what, in 1-3 sentences of Markdown.
- "key" is short, stable kebab-case. "label" is at most 4 words.
- "fileNotes", "lineNotes" and "critical" are optional and sparing. Add a note only where it saves the reviewer time - non-obvious logic, a subtle behavior change, a risk - never to explain the obvious. Mark "critical" only what deserves extra care; most groups and files are not critical.
- Write "label", "summary", "overallSummary" and note "text" in the language named at the end of the user message. Keep code, paths and identifiers as they are.
</output>`;
`${ROLE_SECTION}${OUTPUT_SECTION}`;
/**
* The system prompt for a local agent CLI (`plans/11-local-agents.md`), which has its own
* file tools instead of `read_diffs` and answers with JSON instead of calling `submit_grouping`.
*/
function buildCliAgentSystemPrompt({ patchPath, repository }) {
	return `${ROLE_SECTION}

<workflow>
1. Read the manifest and form a grouping hypothesis from paths and hunk headers.
2. The full patch is at ${patchPath}; read it (or parts of it) only where the hypothesis is uncertain, or where a change looks risky enough to deserve a note. ${repository ? `Open files in the repository when the hunks are not enough to tell what a change is for. Never modify anything.` : `The patch is all there is: the repository is not checked out here.`} Never read [generated] or [binary] paths.
3. Finish by answering with the grouping as JSON matching the schema, and nothing else.
</workflow>

${OUTPUT_SECTION}`;
}
var STATUS_LETTERS = {
	added: "A",
	removed: "D",
	modified: "M",
	renamed: "R",
	copied: "C"
};
function buildHunkContextTag(file) {
	const contexts = [];
	for (const hunk of file.hunks) {
		const parts = hunk.header.split("@@");
		const context = parts.length >= 3 ? parts[2].trim() : "";
		if (!context || contexts.includes(context)) continue;
		contexts.push(context);
		if (contexts.length === 3) break;
	}
	return contexts.map((context) => `@@ ${context}`).join(" / ");
}
function buildManifestFileLine(file) {
	const fields = [
		`${file.path.slice(file.path.lastIndexOf("/") + 1)}${file.previousPath ? ` (from ${file.previousPath})` : ""}`,
		STATUS_LETTERS[file.status],
		`+${file.additions}/-${file.deletions}`
	];
	const tag = isGeneratedPath(file.path) ? "[generated]" : file.isBinary ? "[binary]" : buildHunkContextTag(file);
	if (tag) fields.push(tag);
	return fields.join("  ");
}
function buildManifest(files) {
	const byDir = /* @__PURE__ */ new Map();
	for (const file of [...files].sort((a, b) => a.path.localeCompare(b.path))) {
		const dir = file.path.slice(0, file.path.lastIndexOf("/") + 1);
		const group = byDir.get(dir) ?? [];
		group.push(file);
		byDir.set(dir, group);
	}
	const lines = [];
	for (const dir of [...byDir.keys()].sort()) {
		if (dir) lines.push(dir);
		for (const file of byDir.get(dir)) lines.push(dir ? `  ${buildManifestFileLine(file)}` : buildManifestFileLine(file));
	}
	return lines.join("\n");
}
/** The first user message. Stays English whatever `locale` is; only the closing instruction names the output language. */
function buildAnalysisPrompt(diff, locale) {
	const parts = [`PR title: ${diff.title}`];
	if (diff.url) parts.push(`PR link: ${diff.url}`);
	if (diff.description) parts.push(`---DESCRIPTION---\n${diff.description}`);
	if (diff.commits && diff.commits.length > 1) parts.push(`---COMMITS--- (${diff.commits.length}, oldest first)\n${diff.commits.map((commit) => `${commit.sha.slice(0, 7)} ${commit.message.split("\n", 1)[0]}`).join("\n")}`);
	const totalAdditions = diff.files.reduce((sum, file) => sum + file.additions, 0);
	const totalDeletions = diff.files.reduce((sum, file) => sum + file.deletions, 0);
	parts.push(`---MANIFEST--- (${diff.files.length} files, +${totalAdditions}/-${totalDeletions})\n${buildManifest(diff.files)}`);
	const diffsText = renderFilesAsText(diff.files);
	if (diffsText.length <= 2e5) parts.push(`---DIFFS--- (all diffs included; you may submit directly)\n${diffsText}`);
	parts.push(`Respond and categorize in ${promptLanguageName(locale)}.`);
	return parts.join("\n\n");
}
/**
* Shape the model submits via `submit_grouping`: everything
* `GroupedResult` needs except the fields the adapter itself owns (`source`,
* `schemaVersion`, `generatedAt`).
*/
var AnalysisSchema = /* @__PURE__ */ object({
	overallSummary: /* @__PURE__ */ pipe(/* @__PURE__ */ string(), /* @__PURE__ */ description("A short summary of the intention of the PR (why over what) for a reviewer who hasn't read it yet. Rendered as Markdown.")),
	groups: /* @__PURE__ */ array(withChildren(SubmittedGroupLeafSchema))
});
/** The same shape as JSON Schema, for an agent CLI that validates or is told its final answer. */
function analysisJsonSchema() {
	return toJsonSchema(AnalysisSchema);
}
function findCoverageIssues(diff, analysis) {
	const knownPaths = new Set(diff.files.map((file) => file.path));
	const seen = /* @__PURE__ */ new Set();
	const duplicated = /* @__PURE__ */ new Set();
	const unknown = [];
	const visit = (path) => {
		if (!knownPaths.has(path)) {
			unknown.push(path);
			return;
		}
		if (seen.has(path)) duplicated.add(path);
		seen.add(path);
	};
	for (const group of analysis.groups) {
		group.filePaths.forEach(visit);
		for (const child of group.children ?? []) child.filePaths.forEach(visit);
	}
	return {
		missing: diff.files.map((file) => file.path).filter((path) => !seen.has(path)),
		duplicated: [...duplicated],
		unknown
	};
}
/** The model-facing list of what a grouping got wrong, ending in `fix` - `undefined` when nothing did. */
function describeCoverageIssues(issues, fix) {
	const lines = [];
	if (issues.missing.length > 0) lines.push(`Missing paths: ${issues.missing.join(", ")}`);
	if (issues.duplicated.length > 0) lines.push(`Duplicated paths: ${issues.duplicated.join(", ")}`);
	if (issues.unknown.length > 0) lines.push(`Unknown paths: ${issues.unknown.join(", ")}`);
	if (lines.length === 0) return void 0;
	lines.push(fix);
	return lines.join("\n");
}
toJsonSchema(AnalysisSchema);
//#endregion
//#region ../../node_modules/.pnpm/@pulls.review+core@0.3.1_typescript@6.0.3_ws@8.22.0_zod@4.6.5/node_modules/@pulls.review/core/dist/patch-parser.js
var GIT_DIFF_HEADER_RE = /^diff --git a\/.* b\/(.*)$/;
/** Git quotes a path holding a tab, newline, `"` or `\` (and, without `core.quotePath=false`, any non-ASCII byte). */
var QUOTED_GIT_DIFF_HEADER_RE = /^diff --git "a\/(?:[^"\\]|\\.)*" "b\/((?:[^"\\]|\\.)*)"$/;
var C_ESCAPES = {
	"a": 7,
	"b": 8,
	"t": 9,
	"n": 10,
	"v": 11,
	"f": 12,
	"r": 13,
	"\"": 34,
	"\\": 92
};
/** Reverses git's C-style quoting of a path (`"x\"y"` -> `x"y`); octal escapes are UTF-8 bytes. */
function unquoteGitPath(text) {
	if (!text.startsWith("\"") || !text.endsWith("\"")) return text;
	const bytes = [];
	const body = text.slice(1, -1);
	for (let i = 0; i < body.length; i++) {
		const char = body[i];
		if (char !== "\\") {
			bytes.push(...new TextEncoder().encode(char));
			continue;
		}
		const octal = /^[0-7]{3}/.exec(body.slice(i + 1));
		if (octal) {
			bytes.push(Number.parseInt(octal[0], 8));
			i += 3;
		} else {
			bytes.push(C_ESCAPES[body[i + 1]] ?? body.charCodeAt(i + 1));
			i++;
		}
	}
	return new TextDecoder().decode(new Uint8Array(bytes));
}
var INDEX_LINE_RE = /^index ([0-9a-f]+)\.\.([0-9a-f]+)(?:\s+\d+)?$/;
var HUNK_HEADER_RE = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@.*$/;
var RENAME_FROM_RE = /^rename from (.+)$/;
var RENAME_TO_RE = /^rename to (.+)$/;
var COPY_FROM_RE = /^copy from (.+)$/;
var COPY_TO_RE = /^copy to (.+)$/;
function splitFileChunks(text) {
	if (/^diff --git /m.test(text)) return {
		chunks: text.split(/(?=^diff --git )/m).filter((chunk) => chunk.trim().length > 0),
		isGitDiff: true
	};
	return {
		chunks: text.split(/(?=^--- )/m).filter((chunk) => chunk.trim().length > 0),
		isGitDiff: false
	};
}
/** Parses `@@ ... @@` hunk blocks out of a list of diff lines (with or without a leading `diff --git`/`---`/`+++` preamble). */
function parseHunks(lines) {
	const hunks = [];
	let i = 0;
	while (i < lines.length) {
		const line = lines[i];
		const match = HUNK_HEADER_RE.exec(line);
		if (!match) {
			i++;
			continue;
		}
		const header = line;
		const oldStart = Number(match[1]);
		const oldLines = match[2] === void 0 ? 1 : Number(match[2]);
		const newStart = Number(match[3]);
		const newLines = match[4] === void 0 ? 1 : Number(match[4]);
		i++;
		const bodyLines = [];
		while (i < lines.length && !HUNK_HEADER_RE.test(lines[i])) {
			bodyLines.push(lines[i]);
			i++;
		}
		while (bodyLines.length > 0 && bodyLines[bodyLines.length - 1] === "") bodyLines.pop();
		hunks.push({
			header,
			oldStart,
			oldLines,
			newStart,
			newLines,
			patch: bodyLines.join("\n")
		});
	}
	return hunks;
}
function countChanges(hunks) {
	let additions = 0;
	let deletions = 0;
	for (const hunk of hunks) for (const line of hunk.patch.split("\n")) if (line.startsWith("+")) additions++;
	else if (line.startsWith("-")) deletions++;
	return {
		additions,
		deletions
	};
}
function parseGitPreambleLine(line, preamble) {
	if (line === "new file mode" || /^new file mode \d+$/.test(line)) {
		preamble.isNewFile = true;
		return;
	}
	if (/^deleted file mode \d+$/.test(line)) {
		preamble.isDeletedFile = true;
		return;
	}
	if (line.startsWith("Binary files ") && line.endsWith(" differ")) {
		preamble.isBinary = true;
		return;
	}
	if (line === "GIT binary patch") {
		preamble.isBinary = true;
		return;
	}
	const unquoted = (match) => match?.[1] === void 0 ? void 0 : unquoteGitPath(match[1]);
	preamble.renameFrom ??= unquoted(RENAME_FROM_RE.exec(line));
	preamble.renameTo ??= unquoted(RENAME_TO_RE.exec(line));
	preamble.copyFrom ??= unquoted(COPY_FROM_RE.exec(line));
	preamble.copyTo ??= unquoted(COPY_TO_RE.exec(line));
	const indexMatch = INDEX_LINE_RE.exec(line);
	if (indexMatch) {
		preamble.oldIndexSha = indexMatch[1];
		preamble.newIndexSha = indexMatch[2];
	}
}
function parseGitPreamble(lines) {
	const preamble = {
		isNewFile: false,
		isDeletedFile: false,
		isBinary: false,
		hunkStartIndex: -1
	};
	for (let i = 1; i < lines.length; i++) {
		const line = lines[i];
		if (line.startsWith("@@ ")) {
			preamble.hunkStartIndex = i;
			break;
		}
		parseGitPreambleLine(line, preamble);
	}
	return preamble;
}
function resolveGitStatusAndPath(preamble, headerPath) {
	if (preamble.renameFrom && preamble.renameTo) return {
		status: "renamed",
		previousPath: preamble.renameFrom,
		path: preamble.renameTo
	};
	if (preamble.copyFrom && preamble.copyTo) return {
		status: "copied",
		previousPath: preamble.copyFrom,
		path: preamble.copyTo
	};
	if (preamble.isNewFile) return {
		status: "added",
		path: headerPath
	};
	if (preamble.isDeletedFile) return {
		status: "removed",
		path: headerPath
	};
	return {
		status: "modified",
		path: headerPath
	};
}
function parseGitChunk(chunk) {
	const lines = chunk.split("\n");
	const header = lines[0] ?? "";
	const quoted = QUOTED_GIT_DIFF_HEADER_RE.exec(header)?.[1];
	const headerPath = quoted === void 0 ? GIT_DIFF_HEADER_RE.exec(header)?.[1] ?? "" : unquoteGitPath(`"${quoted}"`);
	const preamble = parseGitPreamble(lines);
	const { status, path, previousPath } = resolveGitStatusAndPath(preamble, headerPath);
	const hunks = preamble.isBinary || preamble.hunkStartIndex === -1 ? [] : parseHunks(lines.slice(preamble.hunkStartIndex));
	return {
		path,
		previousPath,
		status,
		isBinary: preamble.isBinary,
		oldIndexSha: preamble.oldIndexSha,
		newIndexSha: preamble.newIndexSha,
		hunks
	};
}
function parsePosixChunk(chunk) {
	const lines = chunk.split("\n");
	const oldPathLine = lines[0]?.replace(/^--- /, "").split("	")[0] ?? "";
	const newPathLine = lines[1]?.replace(/^\+\+\+ /, "").split("	")[0] ?? "";
	const path = newPathLine === "/dev/null" ? oldPathLine : newPathLine;
	const status = oldPathLine === "/dev/null" ? "added" : newPathLine === "/dev/null" ? "removed" : "modified";
	const hunkStartIndex = lines.findIndex((line) => line.startsWith("@@ "));
	return {
		path,
		status,
		isBinary: false,
		hunks: hunkStartIndex === -1 ? [] : parseHunks(lines.slice(hunkStartIndex))
	};
}
/** SHA-256 hex digest of arbitrary text: the same primitive `parsePatch` uses for its content-hash sha fallback, reused as the `paste` provider's cache key. */
async function computeContentHash(text) {
	const bytes = new TextEncoder().encode(text);
	const digest = await crypto.subtle.digest("SHA-256", bytes);
	return Array.from(new Uint8Array(digest)).map((byte) => byte.toString(16).padStart(2, "0")).join("");
}
/**
* Parses unified-diff / git-extended-diff text (the format shared by GitHub's
* `.diff` endpoint, `git diff` output, and plain `diff -u`) into canonical
* `FileChange`s.
*/
async function parsePatch(text) {
	const { chunks, isGitDiff } = splitFileChunks(text);
	return Promise.all(chunks.map(async (chunk) => {
		const parsed = isGitDiff ? parseGitChunk(chunk) : parsePosixChunk(chunk);
		const { additions, deletions } = countChanges(parsed.hunks);
		let sha;
		if (parsed.status === "removed" && parsed.oldIndexSha) sha = parsed.oldIndexSha;
		else if (parsed.newIndexSha) sha = parsed.newIndexSha;
		else sha = await computeContentHash(chunk);
		return {
			path: parsed.path,
			previousPath: parsed.previousPath,
			status: parsed.status,
			additions,
			deletions,
			isBinary: parsed.isBinary,
			sha,
			hunks: parsed.hunks
		};
	}));
}
//#endregion
//#region src/index.ts
function prompt(diff, patchPath) {
	return `${buildCliAgentSystemPrompt({
		patchPath,
		repository: true
	})}\n\n${buildAnalysisPrompt(diff, "en")}`;
}
function check(diff, answer) {
	const parsed = /* @__PURE__ */ safeParse(AnalysisSchema, answer);
	if (!parsed.success) return { problem: /* @__PURE__ */ summarize(parsed.issues) };
	const problem = describeCoverageIssues(findCoverageIssues(diff, parsed.output), "Answer again with the whole grouping, every path in exactly one group.");
	return problem ? { problem } : { analysis: parsed.output };
}
//#endregion
export { analysisJsonSchema, check, parsePatch, prompt };
