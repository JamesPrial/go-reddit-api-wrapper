<script>
  import { login, APIError } from './api.js';

  // Props
  export let onLoginSuccess;

  // State
  let username = '';
  let password = '';
  let loading = false;
  let error = '';

  // Validation errors
  let usernameError = '';
  let passwordError = '';

  /**
   * Validate form inputs
   */
  function validateForm() {
    let valid = true;

    // Reset errors
    usernameError = '';
    passwordError = '';

    // Validate username
    if (!username.trim()) {
      usernameError = 'Username is required';
      valid = false;
    } else if (username.trim().length < 3) {
      usernameError = 'Username must be at least 3 characters';
      valid = false;
    }

    // Validate password
    if (!password) {
      passwordError = 'Password is required';
      valid = false;
    } else if (password.length < 6) {
      passwordError = 'Password must be at least 6 characters';
      valid = false;
    }

    return valid;
  }

  /**
   * Handle form submission
   */
  async function handleSubmit(event) {
    event.preventDefault();

    // Reset global error
    error = '';

    // Validate inputs
    if (!validateForm()) {
      return;
    }

    // Attempt login
    loading = true;

    try {
      const response = await login(username, password);

      if (response.success && response.token) {
        // Login successful - call parent callback
        onLoginSuccess(response.token, response.username);
      } else {
        error = 'Login failed. Please try again.';
      }
    } catch (err) {
      if (err instanceof APIError) {
        // Handle specific API errors
        if (err.status === 401) {
          error = 'Invalid username or password';
        } else if (err.status === 429) {
          error = 'Too many login attempts. Please try again later.';
        } else if (err.status === 500) {
          error = 'Server error. Please try again later.';
        } else {
          error = err.message || 'Login failed';
        }
      } else {
        error = 'Network error. Please check your connection.';
      }
    } finally {
      loading = false;
    }
  }

  /**
   * Clear field error on input
   */
  function handleUsernameInput() {
    if (usernameError) {
      usernameError = '';
    }
    if (error) {
      error = '';
    }
  }

  function handlePasswordInput() {
    if (passwordError) {
      passwordError = '';
    }
    if (error) {
      error = '';
    }
  }
</script>

<div class="login-container">
  <div class="login-card">
    <h1>Reddit Login</h1>
    <p class="subtitle">Sign in with your Reddit credentials</p>

    <form on:submit={handleSubmit}>
      <!-- Username field -->
      <div class="form-group">
        <label for="username">Username</label>
        <input
          id="username"
          type="text"
          bind:value={username}
          on:input={handleUsernameInput}
          disabled={loading}
          class:error={usernameError}
          placeholder="Enter your Reddit username"
        />
        {#if usernameError}
          <span class="field-error">{usernameError}</span>
        {/if}
      </div>

      <!-- Password field -->
      <div class="form-group">
        <label for="password">Password</label>
        <input
          id="password"
          type="password"
          bind:value={password}
          on:input={handlePasswordInput}
          disabled={loading}
          class:error={passwordError}
          placeholder="Enter your password"
        />
        {#if passwordError}
          <span class="field-error">{passwordError}</span>
        {/if}
      </div>

      <!-- Global error message -->
      {#if error}
        <div class="error-message">
          {error}
        </div>
      {/if}

      <!-- Submit button -->
      <button
        type="submit"
        disabled={loading}
        class="login-button"
      >
        {#if loading}
          <span class="spinner"></span>
          Signing in...
        {:else}
          Sign In
        {/if}
      </button>
    </form>

    <p class="note">
      Note: This uses Reddit's OAuth2 password flow. Your credentials are sent directly to Reddit.
    </p>
  </div>
</div>

<style>
  .login-container {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 100vh;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    padding: 20px;
  }

  .login-card {
    background: white;
    border-radius: 12px;
    padding: 40px;
    max-width: 400px;
    width: 100%;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
  }

  h1 {
    margin: 0 0 8px 0;
    font-size: 28px;
    color: #1a1a1a;
    text-align: center;
  }

  .subtitle {
    margin: 0 0 32px 0;
    text-align: center;
    color: #666;
    font-size: 14px;
  }

  .form-group {
    margin-bottom: 20px;
  }

  label {
    display: block;
    margin-bottom: 8px;
    font-weight: 500;
    color: #333;
    font-size: 14px;
  }

  input {
    width: 100%;
    padding: 12px 16px;
    border: 2px solid #e0e0e0;
    border-radius: 8px;
    font-size: 16px;
    transition: border-color 0.2s;
    box-sizing: border-box;
  }

  input:focus {
    outline: none;
    border-color: #667eea;
  }

  input:disabled {
    background-color: #f5f5f5;
    cursor: not-allowed;
  }

  input.error {
    border-color: #ef4444;
  }

  .field-error {
    display: block;
    margin-top: 6px;
    color: #ef4444;
    font-size: 13px;
  }

  .error-message {
    background-color: #fee;
    border: 1px solid #fcc;
    border-radius: 8px;
    padding: 12px 16px;
    margin-bottom: 20px;
    color: #c33;
    font-size: 14px;
  }

  .login-button {
    width: 100%;
    padding: 14px;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: white;
    border: none;
    border-radius: 8px;
    font-size: 16px;
    font-weight: 600;
    cursor: pointer;
    transition: transform 0.2s, box-shadow 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }

  .login-button:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
  }

  .login-button:active:not(:disabled) {
    transform: translateY(0);
  }

  .login-button:disabled {
    opacity: 0.7;
    cursor: not-allowed;
  }

  .spinner {
    width: 16px;
    height: 16px;
    border: 2px solid #ffffff;
    border-top-color: transparent;
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .note {
    margin-top: 24px;
    text-align: center;
    font-size: 12px;
    color: #999;
    line-height: 1.5;
  }
</style>
