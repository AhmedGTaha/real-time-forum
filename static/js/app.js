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

const createPostForm = document.getElementById("create-post-form");
const postsFeed = document.getElementById("posts-feed");

const commentsPanel = document.getElementById("comments-panel");
const commentsPostTitle = document.getElementById("comments-post-title");
const closeCommentsBtn = document.getElementById("close-comments-btn");
const commentsList = document.getElementById("comments-list");
const createCommentForm = document.getElementById("create-comment-form");
const commentContent = document.getElementById("comment-content");

let selectedPostID = null;

showLoginBtn.addEventListener("click", showLoginForm);
showRegisterBtn.addEventListener("click", showRegisterForm);

loginForm.addEventListener("submit", handleLogin);
registerForm.addEventListener("submit", handleRegister);
logoutBtn.addEventListener("click", handleLogout);

createPostForm.addEventListener("submit", handleCreatePost);
postsFeed.addEventListener("click", handlePostsFeedClick);

closeCommentsBtn.addEventListener("click", closeCommentsPanel);
createCommentForm.addEventListener("submit", handleCreateComment);
commentsList.addEventListener("click", handleCommentsListClick);

checkCurrentUser();

function showLoginForm() {
  loginForm.classList.remove("hidden");
  registerForm.classList.add("hidden");

  showLoginBtn.classList.add("active");
  showRegisterBtn.classList.remove("active");

  clearMessage();
}

function showRegisterForm() {
  registerForm.classList.remove("hidden");
  loginForm.classList.add("hidden");

  showRegisterBtn.classList.add("active");
  showLoginBtn.classList.remove("active");

  clearMessage();
}

async function checkCurrentUser() {
  try {
    const response = await fetch("/api/me");

    if (!response.ok) {
      showGuestView();
      return;
    }

    const data = await response.json();
    showUserView(data.user);
  } catch (error) {
    showGuestView();
  }
}

async function handleRegister(event) {
  event.preventDefault();

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
  event.preventDefault();

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
  const result = await fetch("/api/logout", {
    method: "POST",
  });

  if (!result.ok) {
    showMessage("Logout failed", true);
    return;
  }

  closeCommentsPanel();
  showGuestView();
  showMessage("Logged out successfully", false);
}

async function handleCreatePost(event) {
  event.preventDefault();

  const categoriesInput = inputValue("post-categories");

  const payload = {
    title: inputValue("post-title"),
    content: inputValue("post-content"),
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
  closeCommentsPanel();
  showMessage("Post created successfully", false);
  await loadPosts();
}

async function loadPosts() {
  try {
    const response = await fetch("/api/posts");
    const data = await readResponseJSON(response);

    if (!response.ok) {
      showMessage(data.error || "Failed to load posts", true);
      postsFeed.innerHTML = "<p>Failed to load posts.</p>";
      return;
    }

    renderPosts(data.posts);
  } catch (error) {
    postsFeed.innerHTML = "<p>Network error while loading posts.</p>";
    showMessage("Network error while loading posts", true);
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

    const categories = Array.isArray(post.categories) ? post.categories : [];

    postElement.innerHTML = `
      <div class="post-header">
        <h4>${escapeHTML(post.title)}</h4>
        <span>by ${escapeHTML(post.author)}</span>
      </div>

      <p>${escapeHTML(post.content)}</p>

      <div class="post-categories">
        ${categories.map((category) => `<span>${escapeHTML(category)}</span>`).join("")}
      </div>

      <div class="post-meta">
        <span>${Number(post.like_count) || 0} likes</span>
        <span>${Number(post.comment_count) || 0} comments</span>
        <span>${escapeHTML(post.created_at)}</span>
        <button
          class="like-btn post-like-btn"
          type="button"
          data-post-id="${post.id}"
        >
          Like
        </button>
        <button
          class="view-comments-btn"
          type="button"
          data-post-id="${post.id}"
          data-post-title="${escapeHTML(post.title)}"
        >
          View comments
        </button>
      </div>
    `;

    postsFeed.appendChild(postElement);
  });
}

function handlePostsFeedClick(event) {
  const postLikeButton = event.target.closest(".post-like-btn");

  if (postLikeButton) {
    const postID = Number(postLikeButton.dataset.postId);

    if (!postID) {
      showMessage("Invalid post selected", true);
      return;
    }

    togglePostLike(postID);
    return;
  }

  const commentsButton = event.target.closest(".view-comments-btn");

  if (!commentsButton) {
    return;
  }

  const postID = Number(commentsButton.dataset.postId);
  const postTitle = commentsButton.dataset.postTitle;

  if (!postID) {
    showMessage("Invalid post selected", true);
    return;
  }

  openCommentsPanel(postID, postTitle);
}

async function openCommentsPanel(postID, postTitle) {
  selectedPostID = postID;
  commentsPostTitle.textContent = `Comments: ${postTitle}`;
  commentsPanel.classList.remove("hidden");
  commentContent.value = "";

  await loadComments(postID);
}

function closeCommentsPanel() {
  selectedPostID = null;
  commentsPanel.classList.add("hidden");
  commentsList.innerHTML = "";
  commentContent.value = "";
}

async function loadComments(postID) {
  commentsList.innerHTML = "<p>Loading comments...</p>";

  try {
    const response = await fetch(`/api/comments?post_id=${encodeURIComponent(postID)}`);
    const data = await readResponseJSON(response);

    if (!response.ok) {
      showMessage(data.error || "Failed to load comments", true);
      commentsList.innerHTML = "<p>Failed to load comments.</p>";
      return;
    }

    renderComments(data.comments);
  } catch (error) {
    commentsList.innerHTML = "<p>Network error while loading comments.</p>";
    showMessage("Network error while loading comments", true);
  }
}

function renderComments(comments) {
  commentsList.innerHTML = "";

  if (!comments || comments.length === 0) {
    commentsList.innerHTML = "<p>No comments yet.</p>";
    return;
  }

  comments.forEach((comment) => {
    const commentElement = document.createElement("article");
    commentElement.className = "comment-card";

    commentElement.innerHTML = `
      <div class="comment-header">
        <strong>${escapeHTML(comment.author)}</strong>
        <span>${escapeHTML(comment.created_at)}</span>
      </div>

      <p>${escapeHTML(comment.content)}</p>

      <div class="comment-meta">
        <span>${Number(comment.like_count) || 0} likes</span>
        <button
          class="like-btn comment-like-btn"
          type="button"
          data-comment-id="${comment.id}"
        >
          Like
        </button>
      </div>
    `;

    commentsList.appendChild(commentElement);
  });
}

async function handleCreateComment(event) {
  event.preventDefault();

  if (!selectedPostID) {
    showMessage("No post selected", true);
    return;
  }

  const payload = {
    post_id: selectedPostID,
    content: commentContent.value,
  };

  const result = await sendJSON("/api/comments", payload);

  if (!result.ok) {
    showMessage(result.data.error || "Failed to create comment", true);
    return;
  }

  commentContent.value = "";
  showMessage("Comment created successfully", false);

  await loadComments(selectedPostID);
  await loadPosts();
}

async function sendJSON(url, payload) {
  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    });

    const data = await readResponseJSON(response);

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

async function readResponseJSON(response) {
  try {
    return await response.json();
  } catch (error) {
    return {};
  }
}

function inputValue(id) {
  return document.getElementById(id).value;
}

function showGuestView() {
  showLoginForm();
  closeCommentsPanel();

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

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

async function togglePostLike(postID) {
  const result = await sendJSON("/api/likes/post", {
    post_id: postID,
  });

  if (!result.ok) {
    showMessage(result.data.error || "Failed to like post", true);
    return;
  }

  await loadPosts();

  if (selectedPostID) {
    await loadComments(selectedPostID);
  }
}

function handleCommentsListClick(event) {
  const commentLikeButton = event.target.closest(".comment-like-btn");

  if (!commentLikeButton) {
    return;
  }

  const commentID = Number(commentLikeButton.dataset.commentId);

  if (!commentID) {
    showMessage("Invalid comment selected", true);
    return;
  }

  toggleCommentLike(commentID);
}

async function toggleCommentLike(commentID) {
  const result = await sendJSON("/api/likes/comment", {
    comment_id: commentID,
  });

  if (!result.ok) {
    showMessage(result.data.error || "Failed to like comment", true);
    return;
  }

  if (selectedPostID) {
    await loadComments(selectedPostID);
  }
}
