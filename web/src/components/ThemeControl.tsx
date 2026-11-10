import { LucideIcon, Monitor, Moon, Sun } from "lucide-react";
import { useId } from "react";
import type { Theme } from "../prefs";

const options: { value: Theme; label: string; icon: LucideIcon }[] = [
  { value: "light", label: "Light", icon: Sun },
  { value: "dark", label: "Dark", icon: Moon },
  { value: "system", label: "System", icon: Monitor },
];

/**
 * ThemeControl is the admin theme choice: a fieldset (a radio group named "Theme" by its
 * visually hidden legend) of three native radios, Light, Dark and System, drawn as a segmented
 * control with Sun, Moon and Monitor icons. `theme` is checked; choosing another calls
 * `onChange` with it. It only reports the choice: saving and applying it is the caller's job.
 * The selected segment gets the accent fill with white text (CSS), never accent-colored text.
 */
export default function ThemeControl({ theme, onChange }: { theme: Theme; onChange: (theme: Theme) => void }) {
  const name = useId();
  return (
    <fieldset className="theme-control" role="radiogroup">
      <legend className="sr-only">Theme</legend>
      {options.map((o) => (
        <label key={o.value} className={o.value === theme ? "theme-option selected" : "theme-option"}>
          <input
            type="radio"
            name={name}
            value={o.value}
            checked={o.value === theme}
            onChange={() => onChange(o.value)}
          />
          <o.icon aria-hidden="true" />
          <span>{o.label}</span>
        </label>
      ))}
    </fieldset>
  );
}
