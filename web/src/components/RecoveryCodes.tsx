import { useState } from "react";

/**
 * RecoveryCodes shows a new set of recovery `codes` the one time the API returns them, with a
 * warning that they are not shown again. Copy puts them on the clipboard one per line; Download
 * saves the same text as beer-menu-recovery-codes.txt. Done calls `onDone` and stays disabled
 * until "I saved these codes" is checked, so the codes are not dismissed by accident.
 */
export default function RecoveryCodes({ codes, onDone }: { codes: string[]; onDone: () => void }) {
  const [saved, setSaved] = useState(false);
  const [copied, setCopied] = useState("");
  const text = codes.join("\n");

  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied("Copied.");
    } catch {
      // No clipboard on plain HTTP: the codes stay on screen to copy by hand.
      setCopied("Copy failed. Select the codes and copy them by hand.");
    }
  }

  function download() {
    const url = URL.createObjectURL(new Blob([text + "\n"], { type: "text/plain" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = "beer-menu-recovery-codes.txt";
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div className="secret-box">
      <p>
        Save these recovery codes somewhere safe. They are shown only once. Each one signs you in once if you
        lose your authenticator or passkeys.
      </p>
      <ul className="code-list" aria-label="Recovery codes">
        {codes.map((c) => (
          <li key={c}>
            <code>{c}</code>
          </li>
        ))}
      </ul>
      <p role="status">{copied}</p>
      <div className="btn-row">
        <button type="button" className="btn" onClick={copy}>
          Copy
        </button>
        <button type="button" className="btn" onClick={download}>
          Download
        </button>
      </div>
      <label>
        <input type="checkbox" checked={saved} onChange={(e) => setSaved(e.target.checked)} />I saved these codes
      </label>
      <div className="btn-row">
        <button type="button" className="btn btn-primary" disabled={!saved} onClick={onDone}>
          Done
        </button>
      </div>
    </div>
  );
}
