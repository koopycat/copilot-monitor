<script lang="ts" generics="T extends string">
  interface Option {
    value: T;
    label: string;
  }
  interface Props {
    options: Option[];
    active: T;
    onchange: (value: T) => void;
  }
  let { options, active, onchange }: Props = $props();
</script>

<span class="toggle-group">
  {#each options as opt (opt.value)}
    <button
      class="toggle-btn"
      class:active={active === opt.value}
      onclick={() => onchange(opt.value)}
    >
      {opt.label}
    </button>
  {/each}
</span>
