---
name: frontend-accessibility-review
description: Review UI changes for accessibility, semantic HTML, keyboard interaction, focus management, responsive layout, design token consistency, form errors, and safe client behavior.
license: Apache-2.0
compatibility: "browser UI, HTML/CSS/JS, React/Vue/Svelte/server-rendered interfaces"
allowed-tools: filesystem scm-api diff-analyzer corpus-selector validator
metadata:
  version: "2.0.0"
  owner: "7review"
  review-domain: "frontend-accessibility"
  risk-tier: "high"
---

# Frontend Accessibility Review Skill

## Activation Contract

Use this skill when changed files affect rendered UI, client-side interaction, forms, routing, design systems, component libraries, templates, CSS, browser scripts, or documentation that defines user-facing behavior. Activate even when the diff is visually small if it changes focus order, disabled/loading states, keyboard handling, text layout, or accessible names.

Do not use this skill for backend-only changes unless they directly alter UI schemas, validation errors, or content returned to frontend components.

## Review Algorithm

1. Identify user workflows affected by changed UI files.
2. Derive the interactive contract: entry point, primary action, cancellation path, error path, loading path, and recovery path.
3. Check semantics before styling: correct element choice, accessible names, labels, landmarks, heading order, table/list structure, and ARIA necessity.
4. Check keyboard and focus behavior for every interactive state.
5. Check visual design constraints: contrast, text wrapping, responsive layout, density, target size, overflow, and layout shift.
6. Compare with existing component and design-token patterns before recommending a new UI convention.
7. Validate client-side security and privacy assumptions for rendered content, URLs, local storage, telemetry, and user-provided HTML.
8. Report only issues that are grounded in the changed behavior or an immediately adjacent broken contract.

## Technical Patterns

### Semantic Structure

- Prefer native controls over div/span interactivity. A clickable element that triggers an action should normally be a `button`; navigation should normally be an anchor with a real `href`.
- Every form control needs a programmatic label. Placeholder text is not a label.
- ARIA must repair a real accessibility gap, not override correct native semantics.
- Heading order must preserve page structure; avoid choosing headings only for visual size.
- Dialogs, menus, tabs, comboboxes, tables, and accordions must follow their expected keyboard model.

### Keyboard and Focus

- All actionable UI must be reachable by keyboard in a logical order.
- Focus indicators must be visible and not removed without an equivalent replacement.
- Modal and popover flows must trap focus while open and restore focus to a sensible origin when closed.
- Loading, disabled, and optimistic states must not strand focus or make the next action ambiguous.
- Keyboard shortcuts must not conflict with text entry or browser/assistive technology defaults.

### Responsive Layout and Text Fit

- Text must wrap or truncate deliberately; it must not overlap adjacent controls or disappear behind fixed containers.
- Buttons, tabs, chips, table cells, and sidebars need stable dimensions so dynamic labels do not resize critical controls unexpectedly.
- Mobile layouts must preserve the primary workflow without hiding required actions behind hover-only affordances.
- Avoid viewport-scaled font sizes for UI controls; use component-level responsive constraints instead.

### Forms and Error States

- Validation errors must identify the field, state the problem, and remain available to assistive technology.
- Server-side validation errors must map back to the same visible controls the user can fix.
- Disabled submit buttons need a discoverable reason when they block the workflow.
- Error summaries should link or move focus to the first actionable error for complex forms.

### Client Safety

- Flag unsafe HTML injection, untrusted URL rendering, token exposure in browser code, and sensitive data stored in durable client storage.
- Confirm that analytics or telemetry added to UI flows does not leak secrets, private diffs, access tokens, or personal data.

## Execution Rules

File frontend findings when the diff makes a user-facing interaction less
operable, less truthful, or inconsistent with the product contract. Treat
accessibility, command clarity, streaming state, and configuration feedback as
production behavior. Suppress purely visual preference unless it blocks scanning,
keyboard use, responsive fit, or task completion.

Select the remediation class:

- semantic/ARIA or keyboard fix for interaction defects
- layout/responsiveness fix for clipping, overlap, or unstable controls
- copy/help fix when the UI names behavior incorrectly
- state-handling fix for loading, streaming, error, empty, and success paths

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| UI surface discovery | `filesystem`, `corpus-selector` | Locate components, templates, styles, design tokens, command renderers, and UI docs that define the changed workflow. |
| Behavior comparison | `diff-analyzer`, `scm-api` | Inspect semantic element changes, focus order, keyboard handling, responsive constraints, error paths, and streaming output behavior. |
| Validation | `validator` | Require a concrete user workflow, reproduction path, or accessibility contract before reporting. |

## Escalation Signals

- An interactive element loses keyboard access, label, focus state, or semantic role.
- Long dynamic text, streamed output, or error content can overlap, truncate critical information, or hide next actions.
- Browser/client code renders untrusted content, URLs, secrets, or telemetry without a safety boundary.

## Evidence Standard

Findings should cite the changed component/template and the user action that exposes the issue. Prefer concrete reproduction language: "Tab from X to Y", "open dialog then press Escape", "submit invalid value", or "resize to a narrow viewport". If the issue depends on an external design token or component convention, cite that source too.

## Runtime Integration Checks

- For CLI/TUI output, validate the operator can discover setup/configuration,
  status, interaction, inspection, approval/finalization, and failure flows
  without reading source code when those flows exist.
- Streaming output must expose partial text, terminal completion, cancellation/error states, and authentication failures without corrupting the visible transcript.
- Wizard UX must produce config that matches runtime validation and packaging;
  missing required external dependencies, credentials, model/tool providers, or
  API tokens must be obvious.
- Browser UI findings must connect rendered behavior to semantic HTML, keyboard order, focus movement, and responsive text constraints.

## Review Output Contract

Frame UI findings around a user workflow and observable failure. Include the control, state, viewport or terminal condition, affected user/operator, and a concrete fix using native semantics, stable layout constraints, or clearer command/state text.

## False Positive Checks

- Do not require ARIA when native HTML already exposes the correct role/name/state.
- Do not demand pixel-perfect visual changes unless a design contract or usability failure is present.
- Do not block purely internal refactors unless they change rendered behavior.
- Do not require a full automated accessibility suite for every cosmetic change; request targeted tests when behavior or regressions justify it.

## Review Questions

- Buttons vs links, labels, names, roles, and ARIA misuse
- Keyboard navigation, focus trapping, focus restoration
- Form validation, error messaging, and disabled/loading states
- Color contrast and token drift
- Mobile wrapping, overflow, and layout shifts
- Unsafe HTML injection, URL handling, and client-side secret exposure

## Conditional Operator UI Checks

Use these rows only when the changed repository exposes an operator UI, TUI,
wizard, command surface, or streaming chat/workflow. Do not treat them as
requirements for ordinary product UI:

- Commands must expose status, setup/configuration, chat or interaction, approval/finalization, inspection, and connection failure states without requiring hidden knowledge when those actions exist.
- Streaming chat output must remain readable while partial chunks arrive, provider errors occur, or cancellation happens.
- Wizard-generated configuration must make required production dependencies visible and must match server/runtime validation.
- Approval and publish commands must clearly distinguish draft, awaiting approval, finalized, failed, and superseded states.
- Error messages should identify the failing subsystem, such as config, external auth, model/tool provider, sidecar dependency, package health, or approval state.

## Test Expectations

- Unit tests for command rendering and generated config shape.
- Streaming tests for partial chunks, final completion, and provider errors.
- Layout/text checks for long repository names, run IDs, findings, and error messages where rendering is deterministic.
- Accessibility tests for any browser UI: labels, roles, keyboard behavior, focus restore, contrast-sensitive states, and form error mapping.

## Finding Template

```md
### [severity] Frontend accessibility issue

- Surface: `<component/page/template>`
- User workflow: `<action path>`
- Problem: `<specific broken semantic, keyboard, layout, form, or client-safety contract>`
- Evidence: `<changed lines, design token, component convention, or reproduction path>`
- Expected behavior: `<accessible and usable behavior>`
- Suggested fix: `<native element/pattern/state handling/test>`
```
