import { FormEvent, useState } from "react";
import { Admin, ApiError, apiFetch, errorMessage, isUnauthenticated, SignInPending } from "../api";
import { getPasskey, passkeysSupported } from "../webauthn";

/**
 * LoginForm signs in with POST /api/session and calls `onSignedIn` with the admin. A 401 shows
 * one generic message whatever the cause, and a 429 says how many minutes to wait, from the
 * Retry-After header. When the password is right but a second factor is owed (202
 * `second_factor`), it offers each listed method this page can use, side by side when there are
 * several: an authenticator code sent to POST /api/session/totp, and "Use a passkey", which runs
 * the device prompt for POST /api/session/passkey/options and sends the result to
 * POST /api/session/passkey. Passkeys are offered only at `passkeys_origin` in a capable browser;
 * elsewhere a hint names the address to use. While recovery codes remain, "Use a recovery code"
 * switches to a recovery code sent to POST /api/session/recovery ("Use another method" switches
 * back), and the recovery code is asked for at once when no other method is usable. A wrong
 * code, a refused passkey or a cancelled prompt keeps the step, and an ended pending sign-in
 * (401 `unauthenticated`) returns to the password step. Cancel deletes the pending sign-in with
 * DELETE /api/session/pending. When 2FA is required and the admin has no factor (202 `enroll`)
 * it calls `onEnroll`. `notice` is a message shown when the form first appears.
 */
export default function LoginForm({
  onSignedIn,
  onEnroll,
  notice = "",
}: {
  onSignedIn: (admin: Admin) => void;
  onEnroll: () => void;
  notice?: string;
}) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [second, setSecond] = useState<SignInPending | null>(null);
  const [recovery, setRecovery] = useState(false);
  const [error, setError] = useState(notice);
  const [busy, setBusy] = useState(false);

  const totp = !!second?.methods?.includes("totp");
  const passkeyListed = !!second?.methods?.includes("passkey");
  const passkey = passkeyListed && passkeysSupported(second?.passkeys_origin);
  const otherMethod = totp || passkey;
  const recoveryStep = !!second?.methods?.includes("recovery") && (recovery || !otherMethod);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const reply = await apiFetch<Admin | SignInPending>("POST", "/api/session", { username, password });
      if (!("next" in reply)) onSignedIn(reply);
      else if (reply.next === "second_factor") {
        setPassword("");
        setCode("");
        setRecovery(false);
        setSecond(reply);
      } else onEnroll();
    } catch (err) {
      setError(loginError(err));
    } finally {
      setBusy(false);
    }
  }

  async function submitCode(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      onSignedIn(await apiFetch<Admin>("POST", recoveryStep ? "/api/session/recovery" : "/api/session/totp", { code }));
    } catch (err) {
      stepFailed(err);
    } finally {
      setBusy(false);
    }
  }

  async function signInWithPasskey() {
    setError("");
    setBusy(true);
    try {
      const assertion = await getPasskey(await apiFetch("POST", "/api/session/passkey/options"));
      if (assertion === "cancelled") setError("Passkey sign-in was cancelled.");
      else onSignedIn(await apiFetch<Admin>("POST", "/api/session/passkey", assertion));
    } catch (err) {
      stepFailed(err);
    } finally {
      setBusy(false);
    }
  }

  // stepFailed shows a second-step error; an ended pending sign-in returns to the password step.
  function stepFailed(err: unknown) {
    if (err instanceof ApiError && err.code === "invalid_code") setError("That code did not work.");
    else if (err instanceof ApiError && err.code === "passkey_failed") setError("The passkey was not accepted.");
    else if (isUnauthenticated(err)) {
      setSecond(null);
      setError("Sign-in timed out, start again.");
    } else setError(errorMessage(err));
  }

  function cancel() {
    setSecond(null);
    setError("");
    // Best effort: an unreached server leaves the pending sign-in to expire in 5 minutes.
    apiFetch("DELETE", "/api/session/pending").catch(() => {});
  }

  // switchRecovery moves between the recovery code and the other methods, starting a fresh code.
  function switchRecovery(on: boolean) {
    setRecovery(on);
    setCode("");
    setError("");
  }

  if (second && recoveryStep) {
    return (
      // Keyed forms remount on each step, so autoFocus moves focus off the button that just went away.
      <form key="recovery" className="stack-form" onSubmit={submitCode}>
        <h2>Sign in</h2>
        <p>Enter one of your saved recovery codes. Each code works once.</p>
        <label>
          Recovery code
          <input
            value={code}
            onChange={(e) => setCode(e.target.value)}
            autoComplete="off"
            maxLength={32}
            required
            autoFocus
          />
        </label>
        {otherMethod && (
          <button type="button" className="btn" onClick={() => switchRecovery(false)}>
            Use another method
          </button>
        )}
        {error && <p role="alert">{error}</p>}
        <div className="btn-row">
          <button type="submit" className="btn btn-primary" disabled={busy}>
            Verify
          </button>
          <button type="button" className="btn" onClick={cancel}>
            Cancel
          </button>
        </div>
      </form>
    );
  }

  if (second) {
    return (
      <form key="second" className="stack-form" onSubmit={submitCode}>
        <h2>Sign in</h2>
        {passkey && (
          <button type="button" className="btn" onClick={signInWithPasskey} disabled={busy} autoFocus={!totp}>
            Use a passkey
          </button>
        )}
        {passkeyListed && !passkey && (
          <p>
            {second.passkeys_origin !== location.origin
              ? `Passkeys work only at ${second.passkeys_origin}.`
              : "Passkeys are not available in this browser."}
          </p>
        )}
        {totp && (
          <>
            <p>Enter the code from your authenticator app.</p>
            <label>
              Authenticator code
              <input
                value={code}
                onChange={(e) => setCode(e.target.value)}
                autoComplete="one-time-code"
                inputMode="numeric"
                maxLength={32}
                required
                autoFocus
              />
            </label>
          </>
        )}
        {second.methods?.includes("recovery") && (
          <button type="button" className="btn" onClick={() => switchRecovery(true)}>
            Use a recovery code
          </button>
        )}
        {error && <p role="alert">{error}</p>}
        <div className="btn-row">
          {totp && (
            <button type="submit" className="btn btn-primary" disabled={busy}>
              Verify
            </button>
          )}
          <button type="button" className="btn" onClick={cancel}>
            Cancel
          </button>
        </div>
      </form>
    );
  }

  return (
    <form className="stack-form" onSubmit={submit}>
      <h2>Sign in</h2>
      <label>
        Username
        <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
      </label>
      <label>
        Password
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="current-password"
          required
        />
      </label>
      {error && <p role="alert">{error}</p>}
      <button type="submit" className="btn btn-primary" disabled={busy}>
        Sign in
      </button>
    </form>
  );
}

function loginError(err: unknown): string {
  return isUnauthenticated(err) ? "Wrong username or password." : errorMessage(err);
}
