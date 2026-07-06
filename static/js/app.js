// Grab the HTML elements once, then reuse these variables below.
const authStatus = document.getElementById("auth-status");
const guestView = document.getElementById("guest-view");
const userView = document.getElementById("user-view");
const message = document.getElementById("message");

const loginForm = document.getElementById("login-form");
const registerForm = document.getElementById("register-form");

const showLoginBtn = document.getElementById("show-login-btn");
const showRegisterBtn = document.getElementById("show-register-btn");

const currentUserNickname = document.getElementById("current-user-nickname");
const logoutBtn = document.getElementById("logout-btn");

// Wire browser events to the functions that handle them.
showLoginBtn.addEventListener("click", showLoginForm);
showRegisterBtn.addEventListener("click", showRegisterForm);

loginForm.addEventListener("submit", handleLogin);
registerForm.addEventListener("submit", handleRegister);
logoutBtn.addEventListener("click", handleLogout);

const createPostForm = document.getElementById("create-post-form");
const postsFeed = document.getElementById("posts-feed");

// On page load, ask the backend if the browser already has a valid session.
checkCurrentUser();

// -----------------------------
// Form switching
// -----------------------------

function showLoginForm() {
  // shows the login form
  loginForm.classList.remove("hidden");
  
  // hides the register form
  registerForm.classList.add("hidden");

  showLoginBtn.classList.add("active");
  showRegisterBtn.classList.remove("active");

  clearMessage();
}

function showRegisterForm() {
  // shows the register form
  registerForm.classList.remove("hidden");

  // hides the login form
  loginForm.classList.add("hidden");

  showRegisterBtn.classList.add("active");
  showLoginBtn.classList.remove("active");

  clearMessage();
}

// -----------------------------
// Session/auth requests
// -----------------------------

async function checkCurrentUser() {
  try {
    // GET /api/me uses the session_id cookie, if the browser has one.
    const response = await fetch("/api/me");

    if (!response.ok) {
      showGuestView();
      return;
    }

    // response.json() converts the backend JSON response into a JS object.
    const data = await response.json();
    showUserView(data.user);
  } catch (error) {
    showGuestView();
  }
}

async function handleRegister(event) {
  // Stop the browser from doing a normal page refresh form submit.
  event.preventDefault();

  // This object will become the JSON request body sent to Go.
  // The key names match the json tags in handlers/auth.go.
  const payload = {
    nickname: inputValue("register-nickname"),
    age: Number(inputValue("register-age")),
    gender: inputValue("register-gender"),
    first_name: inputValue("register-first-name"),
    last_name: inputValue("register-last-name"),
    email: inputValue("register-email"),
    password: inputValue("register-password"),
  };

  const result = await sendJSON("/api/register", payload);

  if (!result.ok) {
    showMessage(result.data.error || "Registration failed", true);
    return;
  }

  showMessage("Registration successful. You can now login.", false);
  registerForm.reset();
  showLoginForm();
}

async function handleLogin(event) {
  // Stop the browser from reloading the page.
  event.preventDefault();

  // This becomes JSON like {"identifier":"ahmed","password":"secret123"}.
  const payload = {
    identifier: inputValue("login-identifier"),
    password: inputValue("login-password"),
  };

  const result = await sendJSON("/api/login", payload);

  if (!result.ok) {
    showMessage(result.data.error || "Login failed", true);
    return;
  }

  loginForm.reset();
  await checkCurrentUser();
}

async function handleLogout() {
  // Logout does not need a JSON body. The cookie tells the backend which
  // session to delete.
  const result = await fetch("/api/logout", {
    method: "POST",
  });

  if (!result.ok) {
    showMessage("Logout failed", true);
    return;
  }

  showGuestView();
  showMessage("Logged out successfully", false);
}

async function handleCreatePost(event) {
  event.preventDefault();

  const categoriesInput = document.getElementById("post-categories").value;

  const payload = {
    title: document.getElementById("post-title").value,
    content: document.getElementById("post-content").value,
    categories: categoriesInput
      .split(",")
      .map((category) => category.trim())
      .filter((category) => category !== ""),
  };

  const result = await sendJSON("/api/posts", payload);

  if (!result.ok) {
    showMessage(result.data.error || "Failed to create post", true);
    return;
  }

  createPostForm.reset();
  showMessage("Post created successfully", false);
  await loadPosts();
}

async function loadPosts() {
  try {
    const response = await fetch("/api/posts");

    if (!response.ok) {
      postsFeed.innerHTML = "<p>Failed to load posts.</p>";
      return;
    }

    const data = await response.json();
    renderPosts(data.posts);
  } catch (error) {
    postsFeed.innerHTML = "<p>Network error while loading posts.</p>";
  }
}

function renderPosts(posts) {
  postsFeed.innerHTML = "";

  if (!posts || posts.length === 0) {
    postsFeed.innerHTML = "<p>No posts yet. Create the first one.</p>";
    return;
  }

  posts.forEach((post) => {
    const postElement = document.createElement("article");
    postElement.className = "post-card";

    postElement.innerHTML = `
      <div class="post-header">
        <h4>${escapeHTML(post.title)}</h4>
        <span>by ${escapeHTML(post.author)}</span>
      </div>

      <p>${escapeHTML(post.content)}</p>

      <div class="post-categories">
        ${post.categories.map((category) => `<span>${escapeHTML(category)}</span>`).join("")}
      </div>

      <div class="post-meta">
        <span>${post.like_count} likes</span>
        <span>${post.comment_count} comments</span>
        <span>${escapeHTML(post.created_at)}</span>
      </div>
    `;

    postsFeed.appendChild(postElement);
  });
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

// -----------------------------
// JSON helper
// -----------------------------

async function sendJSON(url, payload) {
  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        // Tell Go that the request body is JSON.
        "Content-Type": "application/json",
      },
      // JSON.stringify turns a JS object into JSON text for the request body.
      body: JSON.stringify(payload),
    });

    // Convert the JSON response from Go back into a JS object.
    const data = await response.json();

    return {
      ok: response.ok,
      data,
    };
  } catch (error) {
    return {
      ok: false,
      data: {
        error: "Network error",
      },
    };
  }
}

// -----------------------------
// UI helpers
// -----------------------------

function inputValue(id) {
  return document.getElementById(id).value;
}

function showGuestView() {
  showLoginForm();

  authStatus.textContent = "Please login or register.";
  guestView.classList.remove("hidden");
  userView.classList.add("hidden");
}

function showUserView(user) {
  authStatus.textContent = "Session active.";
  currentUserNickname.textContent = user.nickname;

  guestView.classList.add("hidden");
  userView.classList.remove("hidden");

  clearMessage();
  loadPosts();
}

function showMessage(text, isError) {
  message.textContent = text;
  message.className = isError ? "error" : "success";
}

function clearMessage() {
  message.textContent = "";
  message.className = "";
}
