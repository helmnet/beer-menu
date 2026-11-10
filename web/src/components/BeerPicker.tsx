import { Search } from "lucide-react";
import { KeyboardEvent, useId, useState } from "react";
import { Beer } from "../api";

/**
 * BeerPicker chooses a beer by typing (research R6): a WAI-ARIA 1.2 combobox, an input named
 * "Beer" with `aria-expanded`, `aria-controls` and `aria-activedescendant`, and a listbox named
 * "Beers" of the given `beers`, newest first (`created_at`, ties by id descending). Typing
 * filters to names containing the typed text, trimmed, ignoring case; nothing matching shows
 * "No beer matches", and there is never a create option. ArrowDown/ArrowUp move the active
 * option (ArrowDown also opens the list), Enter or a click picks it and calls `onChange` with
 * its id, Escape closes the list. While the list is open Escape is stopped and prevented, so
 * it never also closes a surrounding dialog. The picked `value` stays chosen when the filter
 * hides it; leaving the field or Escape shows its name again. `errorId` marks the input
 * invalid and describes it by that element.
 */
export default function BeerPicker({
  beers,
  value,
  onChange,
  errorId,
}: {
  beers: Beer[];
  value: number | null;
  onChange: (id: number) => void;
  errorId?: string;
}) {
  const id = useId();
  const [open, setOpen] = useState(false);
  // null while not filtering: the input then shows the chosen beer's name.
  const [query, setQuery] = useState<string | null>(null);
  const [active, setActive] = useState(-1);

  const chosen = beers.find((b) => b.id === value);
  const text = query?.trim().toLocaleLowerCase() ?? "";
  const shown = beers
    .filter((b) => b.name.toLocaleLowerCase().includes(text))
    .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at) || b.id - a.id);
  const optionId = (b: Beer) => `${id}-beer-${b.id}`;

  function close() {
    setOpen(false);
    setQuery(null);
    setActive(-1);
  }

  function pick(b: Beer) {
    onChange(b.id);
    close();
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      setOpen(true);
      const last = shown.length - 1;
      setActive(e.key === "ArrowDown" ? Math.min(active + 1, last) : Math.max(active - 1, 0));
    } else if (e.key === "Enter" && open && shown[active]) {
      e.preventDefault();
      pick(shown[active]);
    } else if (e.key === "Escape" && open) {
      e.preventDefault();
      e.stopPropagation();
      close();
    }
  }

  return (
    <div className="beer-picker">
      <label htmlFor={`${id}-input`}>Beer</label>
      <div className="picker-field">
        <Search aria-hidden="true" />
        <input
          id={`${id}-input`}
          role="combobox"
          autoComplete="off"
          aria-autocomplete="list"
          aria-expanded={open}
          aria-controls={`${id}-list`}
          aria-activedescendant={open && shown[active] ? optionId(shown[active]) : undefined}
          aria-invalid={errorId ? true : undefined}
          aria-describedby={errorId}
          placeholder="Search beers"
          value={query ?? chosen?.name ?? ""}
          onChange={(e) => {
            setQuery(e.target.value);
            setActive(-1);
            setOpen(true);
          }}
          onClick={() => setOpen(true)}
          onKeyDown={onKeyDown}
          onBlur={close}
        />
      </div>
      {/* Pressing in the list must not blur the input first, or the click would land on nothing. */}
      <ul id={`${id}-list`} role="listbox" aria-label="Beers" hidden={!open} onMouseDown={(e) => e.preventDefault()}>
        {shown.map((b, i) => (
          <li
            key={b.id}
            id={optionId(b)}
            role="option"
            aria-selected={b.id === value}
            className={i === active ? "active" : undefined}
            onClick={() => pick(b)}
          >
            {b.name}
          </li>
        ))}
      </ul>
      {open && shown.length === 0 && <p className="muted picker-empty">No beer matches</p>}
    </div>
  );
}
