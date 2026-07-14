<script lang="ts">
  import { Combobox } from 'bits-ui';
  import { dashboard } from '../stores/dashboard.svelte';

  let editing = $state(false);
  let editMode: 'allow_all' | 'allowlist' | 'blocklist' = $state(dashboard.policy.mode);
  let models: string[] = $state([...dashboard.policy.models]);
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');

  // --- autocomplete state ---
  let inputText = $state('');
  let activeIndex = $state(-1);
  let listOpen = $state(false);
  let inputFocused = $state(false);
  let inputEl: HTMLInputElement | null = $state(null);

  // Filtered suggestions: prefix match, suppress on wildcard or empty input
  const suggestions = $derived.by(() => {
    if (!inputText || inputText.includes('*')) return [];
    const lower = inputText.toLowerCase();
    return dashboard.policyModels
      .filter((m) => m.toLowerCase().startsWith(lower) && !models.includes(m))
      .slice(0, 8);
  });

  // Bits UI owns highlighting and scrolling. Keep the menu synchronized with
  // the same suggestion rules as the previous creatable input.
  $effect(() => {
    if (suggestions.length === 0) {
      listOpen = false;
      activeIndex = -1;
    } else if (inputFocused) {
      listOpen = true;
    }
  });

  function startEdit() {
    editMode = dashboard.policy.mode;
    models = [...dashboard.policy.models];
    inputText = '';
    saved = false;
    error = '';
    activeIndex = -1;
    listOpen = false;
    inputFocused = false;
    editing = true;
    // Focus input after DOM update
    setTimeout(() => inputEl?.focus(), 0);
  }

  function cancelEdit() {
    editing = false;
  }

  function addModel(value: string) {
    const trimmed = value.trim();
    if (!trimmed || models.includes(trimmed)) return;
    models = [...models, trimmed];
    inputText = '';
  }

  function removeModel(index: number) {
    models = models.filter((_, i) => i !== index);
    inputEl?.focus();
  }

  function onComboboxValueChange(value: string[]) {
    models = value;
    inputText = '';
    activeIndex = -1;
  }

  function onComboboxOpenChange(open: boolean) {
    listOpen = open;
    if (!open) activeIndex = -1;
  }

  function onSuggestionHighlight(index: number) {
    activeIndex = index;
  }

  function onInputKeydown(e: KeyboardEvent) {
    // Bits UI handles listbox navigation and selection. Enter remains
    // creatable when it has not highlighted an option.
    if (e.key === 'Enter') {
      const hasActiveSuggestion =
        listOpen && Boolean(inputEl?.getAttribute('aria-activedescendant'));
      if (!hasActiveSuggestion && inputText.trim()) {
        e.preventDefault();
        addModel(inputText);
      }
    } else if (e.key === 'Escape') {
      if (listOpen) {
        e.preventDefault();
        listOpen = false;
        activeIndex = -1;
      } else {
        inputText = '';
      }
    } else if (e.key === 'Backspace' && !inputText && models.length > 0) {
      models = models.slice(0, -1);
    } else if (e.key === ',') {
      e.preventDefault();
      if (inputText.trim()) addModel(inputText);
    }
  }

  function onInputBlur() {
    inputFocused = false;
    // Delay to allow click on suggestion to fire first
    setTimeout(() => {
      if (inputText.trim()) addModel(inputText);
      listOpen = false;
      activeIndex = -1;
    }, 150);
  }

  async function save() {
    saving = true;
    error = '';
    const ok = await dashboard.savePolicy({ mode: editMode, models });
    saving = false;
    if (ok) {
      saved = true;
      editing = false;
    } else {
      error = 'Failed to save policy.';
    }
  }

  function modeLabel(m: string): string {
    return m.replace(/_/g, ' ');
  }
</script>

<section class="policy-panel">
  <h2>Security Policy</h2>

  {#if !editing}
    <div class="policy-summary">
      <span class="tag policy-mode">{modeLabel(dashboard.policy.mode)}</span>
      {#if dashboard.policy.models.length > 0}
        <span class="policy-count"
          >{dashboard.policy.models.length} pattern{dashboard.policy.models.length !== 1
            ? 's'
            : ''}</span
        >
      {/if}
      <button class="btn-sm" onclick={startEdit}>Edit</button>
    </div>
  {:else}
    <div class="toggle-group">
      <label>
        <input type="radio" name="policyMode" value="allow_all" bind:group={editMode} />
        Allow All
      </label>
      <label>
        <input type="radio" name="policyMode" value="blocklist" bind:group={editMode} />
        Block List
      </label>
      <label>
        <input type="radio" name="policyMode" value="allowlist" bind:group={editMode} />
        Allow List
      </label>
    </div>

    {#if editMode !== 'allow_all'}
      <div class="token-input-wrap">
        <Combobox.Root
          type="multiple"
          value={models}
          open={listOpen}
          inputValue={inputText}
          loop={false}
          scrollAlignment="nearest"
          onValueChange={onComboboxValueChange}
          onOpenChange={onComboboxOpenChange}
        >
          <div class="token-input" class:token-input-focus={listOpen || inputFocused}>
            {#each models as model, i (model)}
              <span class="token-chip">
                {model}
                <button
                  class="token-remove"
                  onclick={() => removeModel(i)}
                  aria-label="Remove {model}">&times;</button
                >
              </span>
            {/each}
            <Combobox.Input
              bind:ref={inputEl}
              type="text"
              class="token-input-field"
              oninput={(e) => (inputText = e.currentTarget.value)}
              onkeydown={onInputKeydown}
              onblur={onInputBlur}
              onfocus={() => {
                inputFocused = true;
                if (suggestions.length > 0) listOpen = true;
              }}
              placeholder={models.length === 0 ? 'Type a model name or pattern…' : 'Add model…'}
              aria-label="Model patterns"
              aria-controls="policy-listbox"
              autocomplete="off"
            />
          </div>

          {#if suggestions.length > 0}
            <Combobox.Portal>
              <Combobox.Content
                id="policy-listbox"
                class="model-listbox"
                aria-label="Model suggestions"
                sideOffset={4}
              >
                <Combobox.Viewport>
                  {#each suggestions as suggestion, i (suggestion)}
                    <Combobox.Item
                      value={suggestion}
                      label={suggestion}
                      class={activeIndex === i
                        ? 'model-listbox-option active'
                        : 'model-listbox-option'}
                      onHighlight={() => onSuggestionHighlight(i)}
                      onUnhighlight={() => {
                        if (activeIndex === i) activeIndex = -1;
                      }}
                    >
                      {suggestion}
                    </Combobox.Item>
                  {/each}
                </Combobox.Viewport>
              </Combobox.Content>
            </Combobox.Portal>
          {/if}
        </Combobox.Root>
      </div>
      <p class="token-hint">
        Type to search known models, or enter any pattern. Use <code>*</code> for prefix matching
        (e.g. <code>gpt-*</code>).
      </p>
    {/if}

    <div class="policy-actions">
      <button class="btn-save" onclick={save} disabled={saving}>
        {saving ? 'Saving...' : 'Save'}
      </button>
      <button class="btn-cancel" onclick={cancelEdit}>Cancel</button>
      {#if error}
        <span class="policy-error">{error}</span>
      {/if}
    </div>
  {/if}

  {#if saved}
    <div class="policy-saved">&#10003; Policy updated</div>
  {/if}
</section>

<style>
  /* Bits UI positions the content and owns the viewport scrolling. */
  :global(.model-listbox) {
    max-height: 180px;
  }

  :global(.model-listbox .model-listbox-option[data-highlighted]) {
    background: var(--accent-fill);
    color: var(--on-accent);
  }
</style>
