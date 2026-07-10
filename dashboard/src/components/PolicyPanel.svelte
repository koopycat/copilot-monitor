<script lang="ts">
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
  let inputEl: HTMLInputElement | undefined = $state();
  let listboxEl: HTMLUListElement | undefined = $state();

  // Filtered suggestions: prefix match, suppress on wildcard or empty input
  const suggestions = $derived.by(() => {
    if (!inputText || inputText.includes('*')) return [];
    const lower = inputText.toLowerCase();
    return dashboard.policyModels
      .filter((m) => m.toLowerCase().startsWith(lower) && !models.includes(m))
      .slice(0, 8);
  });

  // Re-sync activeIndex when suggestions change
  $effect(() => {
    if (suggestions.length > 0) {
      activeIndex = 0;
      listOpen = true;
    } else {
      activeIndex = -1;
      listOpen = false;
    }
  });

  // Scroll active option into view
  $effect(() => {
    if (listboxEl && activeIndex >= 0 && listOpen) {
      const opt = listboxEl.children[activeIndex] as HTMLElement | undefined;
      opt?.scrollIntoView({ block: 'nearest' });
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

  function acceptSuggestion(index: number) {
    if (suggestions[index]) {
      addModel(suggestions[index]);
    }
  }

  function onInputKeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (listOpen) {
        activeIndex = Math.min(activeIndex + 1, suggestions.length - 1);
      } else if (suggestions.length > 0) {
        listOpen = true;
        activeIndex = 0;
      }
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (listOpen) {
        activeIndex = Math.max(activeIndex - 1, 0);
      }
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (listOpen && activeIndex >= 0) {
        acceptSuggestion(activeIndex);
      } else if (inputText.trim()) {
        addModel(inputText);
      }
    } else if (e.key === 'Escape') {
      if (listOpen) {
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
    // Delay to allow click on suggestion to fire first
    setTimeout(() => {
      if (inputText.trim()) addModel(inputText);
      listOpen = false;
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
        <div
          class="token-input"
          class:token-input-focus={listOpen || document.activeElement === inputEl}
        >
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
          <input
            bind:this={inputEl}
            type="text"
            class="token-input-field"
            bind:value={inputText}
            onkeydown={onInputKeydown}
            onblur={onInputBlur}
            onfocus={() => {
              if (suggestions.length > 0) listOpen = true;
            }}
            placeholder={models.length === 0 ? 'Type a model name or pattern…' : 'Add model…'}
            role="combobox"
            aria-autocomplete="list"
            aria-expanded={listOpen}
            aria-controls="policy-listbox"
            aria-activedescendant={listOpen && activeIndex >= 0
              ? `policy-opt-${activeIndex}`
              : undefined}
            autocomplete="off"
          />
        </div>
        {#if listOpen && suggestions.length > 0}
          <ul
            bind:this={listboxEl}
            id="policy-listbox"
            class="model-listbox"
            role="listbox"
            aria-label="Model suggestions"
          >
            {#each suggestions as suggestion, i}
              <li
                id="policy-opt-{i}"
                role="option"
                aria-selected={i === activeIndex}
                class="model-listbox-option"
                class:active={i === activeIndex}
                onmousedown={(e) => {
                  e.preventDefault();
                  acceptSuggestion(i);
                }}
              >
                {suggestion}
              </li>
            {/each}
          </ul>
        {/if}
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
