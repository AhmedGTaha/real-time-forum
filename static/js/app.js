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
const feedTitle = document.getElementById("feed-title");
const viewLinks = document.querySelectorAll("[data-view]");

const adminPanel = document.getElementById("admin-panel");
const adminUsersList = document.getElementById("admin-users");
const adminPostsList = document.getElementById("admin-posts");
const adminCommentsList = document.getElementById("admin-comments");
const adminRefreshBtn = document.getElementById("admin-refresh-btn");
const adminLinks = document.querySelectorAll(".admin-link");
const adminOnlyElements = document.querySelectorAll(".admin-only");

const commentsPanel = document.getElementById("comments-panel");
const commentsPostTitle = document.getElementById("comments-post-title");
const closeCommentsBtn = document.getElementById("close-comments-btn");
const commentsList = document.getElementById("comments-list");
const createCommentForm = document.getElementById("create-comment-form");
const commentContent = document.getElementById("comment-content");

const chatUsersList = document.getElementById("chat-users-list");
const chatMessages = document.getElementById("chat-messages");
const chatTitle = document.getElementById("chat-title");
const chatForm = document.getElementById("chat-form");
const chatInput = document.getElementById("chat-input");

const themeToggleButtons = document.querySelectorAll("[data-theme-toggle]");
const mobileMenuButton = document.getElementById("mobile-menu-btn");
const mobileDrawer = document.getElementById("mobile-drawer");
const mobileDrawerClose = document.getElementById("mobile-drawer-close");
const mobileDrawerLinks = document.querySelectorAll("#mobile-drawer a");
const mobileDrawerBackdrop = document.getElementById("sidebar-backdrop");
const headerSearch = document.getElementById("header-search");

let selectedPostID = null;

let currentUser = null;
let currentPosts = [];
let currentUserComments = [];
let currentView = "home";
let lastFocusedElement = null;
let selectedChatUserID = null;
let selectedChatUserOnline = false;
let chatSocket = null;
let oldestMessageID = null;
let isLoadingOlderMessages = false;
let chatScrollTimeout = null;
let displayedChatMessageIDs = new Set();
let unreadChatUserIDs = new Set();
let revealObserver = null;

showLoginBtn.addEventListener("click", showLoginForm);
showRegisterBtn.addEventListener("click", showRegisterForm);

loginForm.addEventListener("submit", handleLogin);
registerForm.addEventListener("submit", handleRegister);
logoutBtn.addEventListener("click", handleLogout);

createPostForm.addEventListener("submit", handleCreatePost);
postsFeed.addEventListener("click", handlePostsFeedClick);

viewLinks.forEach((link) => link.addEventListener("click", handleViewLinkClick));

adminLinks.forEach((link) => link.addEventListener("click", handleAdminLinkClick));
adminRefreshBtn?.addEventListener("click", loadAdminOverview);
adminPanel?.addEventListener("click", handleAdminPanelClick);

closeCommentsBtn.addEventListener("click", closeCommentsPanel);
createCommentForm.addEventListener("submit", handleCreateComment);
commentsList.addEventListener("click", handleCommentsListClick);

chatUsersList.addEventListener("click", handleChatUserClick);
chatForm.addEventListener("submit", handleSendChatMessage);
chatMessages.addEventListener("scroll", handleChatMessagesScroll);
headerSearch?.addEventListener("input", handlePostSearch);

setupTheme();
setupMobileDrawer();
setupRevealObserver();
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

    currentPosts = Array.isArray(data.posts) ? data.posts : [];

    // Only paint the feed if a post view is active; comment views own it otherwise.
    if (!isCommentView(currentView)) {
      renderPosts(filterPosts(getViewPosts()));
    }
  } catch (error) {
    postsFeed.innerHTML = "<p>Network error while loading posts.</p>";
    showMessage("Network error while loading posts", true);
  }
}

// Sidebar links carry a data-view. Switching view changes what the feed shows:
// all posts, your posts, posts you liked, your comments, or comments you liked.
const FEED_TITLES = {
  "home": "Posts",
  "my-posts": "My posts",
  "liked-posts": "Liked posts",
  "my-comments": "My comments",
  "liked-comments": "Liked comments",
};

function isCommentView(view) {
  return view === "my-comments" || view === "liked-comments";
}

function handleViewLinkClick(event) {
  // Let the anchor still scroll to #main-feed; we only swap the feed contents.
  selectView(event.currentTarget.dataset.view);
}

function selectView(view) {
  currentView = view;

  viewLinks.forEach((link) => {
    link.classList.toggle("active", link.dataset.view === view);
  });

  if (feedTitle) {
    feedTitle.textContent = FEED_TITLES[view] || "Posts";
  }

  loadFeed();
}

// loadFeed fetches whatever the active view needs and renders it.
async function loadFeed() {
  if (isCommentView(currentView)) {
    await loadUserComments();
  } else {
    await loadPosts();
  }
}

// renderCurrentView re-renders from cached data (used by search, no refetch).
function renderCurrentView() {
  if (isCommentView(currentView)) {
    renderUserComments(filterUserComments(currentUserComments));
  } else {
    renderPosts(filterPosts(getViewPosts()));
  }
}

// getViewPosts narrows the cached posts to the active post view.
function getViewPosts() {
  if (currentView === "liked-posts") {
    return currentPosts.filter((post) => post.liked);
  }

  if (currentView === "my-posts") {
    return currentPosts.filter((post) => Number(post.author_id) === getCurrentUserID());
  }

  return currentPosts;
}

async function loadUserComments() {
  postsFeed.className = "timeline";
  postsFeed.innerHTML = "<p>Loading comments...</p>";

  const endpoint = currentView === "liked-comments"
    ? "/api/comments/liked"
    : "/api/comments/mine";

  try {
    const response = await fetch(endpoint);
    const data = await readResponseJSON(response);

    if (!response.ok) {
      showMessage(data.error || "Failed to load comments", true);
      postsFeed.innerHTML = "<p>Failed to load comments.</p>";
      return;
    }

    currentUserComments = Array.isArray(data.comments) ? data.comments : [];
    renderUserComments(filterUserComments(currentUserComments));
  } catch (error) {
    postsFeed.innerHTML = "<p>Network error while loading comments.</p>";
    showMessage("Network error while loading comments", true);
  }
}

function filterUserComments(comments) {
  const query = headerSearch?.value.trim().toLowerCase() || "";

  if (!query) {
    return comments;
  }

  return comments.filter((comment) => {
    const searchableText = `${comment.content} ${comment.author} ${comment.post_title}`.toLowerCase();
    return searchableText.includes(query);
  });
}

function renderUserComments(comments) {
  postsFeed.className = "timeline";
  postsFeed.innerHTML = "";

  if (!comments || comments.length === 0) {
    postsFeed.innerHTML = '<p class="empty-state">No comments to show.</p>';
    return;
  }

  comments.forEach((comment) => {
    const commentElement = document.createElement("article");
    commentElement.className = "comment-card reveal";

    const canDeleteComment = Number(comment.author_id) === getCurrentUserID();
    const deleteCommentButton = canDeleteComment
      ? `
        <button
          class="delete-btn delete-comment-btn"
          type="button"
          data-comment-id="${comment.id}"
        >
          Delete
        </button>
      `
      : "";

    commentElement.innerHTML = `
      <div class="comment-header">
        <strong>${escapeHTML(comment.author)}</strong>
        <span class="mono">${escapeHTML(comment.created_at)}</span>
      </div>

      <p class="comment-context">
        on
        <a class="post-title-link" href="#comments-panel" data-post-id="${comment.post_id}" data-post-title="${escapeHTML(comment.post_title)}">
          ${escapeHTML(comment.post_title)}
        </a>
      </p>

      <p>${escapeHTML(comment.content)}</p>

      <div class="comment-meta">
        <span>${Number(comment.like_count) || 0} likes</span>
        <button
          class="like-btn comment-like-btn${comment.liked ? " liked" : ""}"
          type="button"
          data-comment-id="${comment.id}"
        >
          ${comment.liked ? "Unlike" : "Like"}
        </button>
        ${deleteCommentButton}
      </div>
    `;

    postsFeed.appendChild(commentElement);
    observeRevealElement(commentElement);
  });
}

function renderPosts(posts) {
  postsFeed.className = "post-list";
  postsFeed.innerHTML = "";

  if (!posts || posts.length === 0) {
    postsFeed.innerHTML = '<p class="empty-state">No posts found.</p>';
    return;
  }

  posts.forEach((post) => {
    const postElement = document.createElement("article");
    postElement.className = "post-row reveal";

    const categories = Array.isArray(post.categories) ? post.categories : [];
    const canDeletePost = Number(post.author_id) === getCurrentUserID();
    const deletePostButton = canDeletePost
      ? `
        <button
          class="delete-btn delete-post-btn"
          type="button"
          data-post-id="${post.id}"
        >
          Delete
        </button>
      `
      : "";

    postElement.innerHTML = `
      <div class="post-row-icon" aria-hidden="true">
        <svg width="16" height="16" viewBox="0 0 16 16">
          <circle cx="8" cy="8" r="6" fill="none" stroke="currentColor" stroke-width="2"></circle>
        </svg>
      </div>

      <div class="post-row-body">
        <div class="post-row-title">
          <a class="post-title-link" href="#comments-panel" data-post-id="${post.id}" data-post-title="${escapeHTML(post.title)}">
            ${escapeHTML(post.title)}
          </a>
          ${categories.map((category) => `<span class="category-tag">${escapeHTML(category)}</span>`).join("")}
        </div>

        <p class="post-row-content">${escapeHTML(post.content)}</p>

        <div class="post-row-meta">
          <span>#${Number(post.id) || 0}</span>
          <span>opened ${escapeHTML(post.created_at)} by ${escapeHTML(post.author)}</span>
        </div>

        <div class="post-actions">
          <button
            class="like-btn post-like-btn${post.liked ? " liked" : ""}"
            type="button"
            data-post-id="${post.id}"
          >
            ${post.liked ? "Unlike" : "Like"}
          </button>
          <button
            class="view-comments-btn"
            type="button"
            data-post-id="${post.id}"
            data-post-title="${escapeHTML(post.title)}"
          >
            Comments
          </button>
          ${deletePostButton}
        </div>
      </div>

      <div class="post-row-stats">
        <span title="Comments">${Number(post.comment_count) || 0} comments</span>
        <span title="Likes">${Number(post.like_count) || 0} likes</span>
      </div>
    `;

    postsFeed.appendChild(postElement);
    observeRevealElement(postElement);
  });
}

function handlePostSearch() {
  renderCurrentView();
}

function filterPosts(posts) {
  const query = headerSearch?.value.trim().toLowerCase() || "";

  if (!query) {
    return posts;
  }

  return posts.filter((post) => {
    const categories = Array.isArray(post.categories) ? post.categories.join(" ") : "";
    const searchableText = `${post.title} ${post.content} ${post.author} ${categories}`.toLowerCase();
    return searchableText.includes(query);
  });
}

function handlePostsFeedClick(event) {
  const deletePostButton = event.target.closest(".delete-post-btn");

  if (deletePostButton) {
    const postID = Number(deletePostButton.dataset.postId);

    if (!postID) {
      showMessage("Invalid post selected", true);
      return;
    }

    deletePost(postID);
    return;
  }

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

  // The comment views (My comments / Liked comments) render comment cards into
  // this same feed, so handle their like/delete buttons here too.
  const deleteCommentButton = event.target.closest(".delete-comment-btn");

  if (deleteCommentButton) {
    const commentID = Number(deleteCommentButton.dataset.commentId);

    if (!commentID) {
      showMessage("Invalid comment selected", true);
      return;
    }

    deleteComment(commentID);
    return;
  }

  const commentLikeButton = event.target.closest(".comment-like-btn");

  if (commentLikeButton) {
    const commentID = Number(commentLikeButton.dataset.commentId);

    if (!commentID) {
      showMessage("Invalid comment selected", true);
      return;
    }

    toggleCommentLike(commentID);
    return;
  }

  const commentsButton = event.target.closest(".view-comments-btn, .post-title-link");

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
    commentElement.className = "comment-card reveal";
    const canDeleteComment = Number(comment.author_id) === getCurrentUserID();
    const deleteCommentButton = canDeleteComment
      ? `
        <button
          class="delete-btn delete-comment-btn"
          type="button"
          data-comment-id="${comment.id}"
        >
          Delete
        </button>
      `
      : "";

    commentElement.innerHTML = `
      <div class="comment-header">
        <strong>${escapeHTML(comment.author)}</strong>
        <span class="mono">${escapeHTML(comment.created_at)}</span>
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
        ${deleteCommentButton}
      </div>
    `;

    commentsList.appendChild(commentElement);
    observeRevealElement(commentElement);
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
  return sendJSONRequest(url, "POST", payload);
}

async function sendJSONRequest(url, method, payload) {
  try {
    const response = await fetch(url, {
      method,
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
  currentUser = null;

  showLoginForm();
  closeCommentsPanel();
  closeChatSocket();
  clearChatState();

  authStatus.textContent = "Please login or register.";
  guestView.classList.remove("hidden");
  userView.classList.add("hidden");
}

function showUserView(user) {
  currentUser = user;

  authStatus.textContent = "Session active.";
  currentUserNickname.textContent = user.nickname;
  updateProfileSummary(user);
  updateAdminVisibility(user);

  guestView.classList.add("hidden");
  userView.classList.remove("hidden");

  clearMessage();

  selectView("home");
  loadChatUsers();
  connectChatSocket();
}

function showMessage(text, isError) {
  message.textContent = text;
  message.className = `message-line ${isError ? "error" : "success"}`;
}

function clearMessage() {
  message.textContent = "";
  message.className = "message-line";
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function initials(value) {
  return String(value || "?").trim().slice(0, 2).toUpperCase();
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
  const deleteCommentButton = event.target.closest(".delete-comment-btn");

  if (deleteCommentButton) {
    const commentID = Number(deleteCommentButton.dataset.commentId);

    if (!commentID) {
      showMessage("Invalid comment selected", true);
      return;
    }

    deleteComment(commentID);
    return;
  }

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

async function deletePost(postID) {
  if (!confirm("Delete this post?")) {
    return;
  }

  const result = await sendJSONRequest("/api/posts", "DELETE", {
    post_id: postID,
  });

  if (!result.ok) {
    showMessage(result.data.error || "Failed to delete post", true);
    return;
  }

  if (selectedPostID === postID) {
    closeCommentsPanel();
  }

  showMessage(result.data.message || "Post deleted successfully", false);
  await loadPosts();
}

async function deleteComment(commentID) {
  if (!confirm("Delete this comment?")) {
    return;
  }

  const result = await sendJSONRequest("/api/comments", "DELETE", {
    comment_id: commentID,
  });

  if (!result.ok) {
    showMessage(result.data.error || "Failed to delete comment", true);
    return;
  }

  showMessage(result.data.message || "Comment deleted successfully", false);

  if (selectedPostID) {
    await loadComments(selectedPostID);
  }

  if (isCommentView(currentView)) {
    await loadUserComments();
  }

  await loadPosts();
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

  if (isCommentView(currentView)) {
    await loadUserComments();
  }
}

async function loadChatUsers() {
  try {
    const response = await fetch("/api/chat/users");
    const data = await readResponseJSON(response);

    if (!response.ok) {
      chatUsersList.innerHTML = "<p>Failed to load users.</p>";
      showMessage(data.error || "Failed to load chat users", true);
      return;
    }

    renderChatUsers(data.users);
  } catch (error) {
    chatUsersList.innerHTML = "<p>Network error while loading users.</p>";
    showMessage("Network error while loading chat users", true);
  }
}

function renderChatUsers(users) {
  chatUsersList.innerHTML = "";

  if (!users || users.length === 0) {
    chatUsersList.innerHTML = "<p>No other users yet.</p>";
    selectedChatUserID = null;
    selectedChatUserOnline = false;
    chatTitle.textContent = "Select a user";
    chatMessages.innerHTML = "<p>Select a user to start chatting.</p>";
    updateChatInputState();
    return;
  }

  let selectedUserStillExists = false;

  users.forEach((user) => {
    const userButton = document.createElement("button");
    userButton.type = "button";
    userButton.className = "chat-user-btn";
    userButton.dataset.userId = user.id;
    userButton.dataset.nickname = user.nickname;
    userButton.dataset.online = user.online ? "true" : "false";

    if (Number(user.id) === selectedChatUserID) {
      userButton.classList.add("active");
      selectedChatUserOnline = Boolean(user.online);
      selectedUserStillExists = true;
    }

    const statusClass = user.online ? "online" : "offline";
    const statusText = user.online ? "Online" : "Offline";

    if (!user.online) {
      userButton.classList.add("offline-user");
      userButton.setAttribute("aria-disabled", "true");
    }

    const hasUnread = unreadChatUserIDs.has(Number(user.id));

    userButton.innerHTML = `
      <span class="chat-user-avatar" aria-hidden="true">${escapeHTML(initials(user.nickname))}</span>

      <span class="chat-user-main">
        <span class="chat-user-name">${escapeHTML(user.nickname)}</span>
        <span class="chat-user-meta">
          <small class="${statusClass}">${statusText}</small>
          ${hasUnread ? '<strong class="unread-badge">New</strong>' : ""}
        </span>
      </span>
    `;

    chatUsersList.appendChild(userButton);
  });

  if (selectedChatUserID && !selectedUserStillExists) {
    selectedChatUserID = null;
    selectedChatUserOnline = false;
    chatTitle.textContent = "Select a user";
    chatMessages.innerHTML = "<p>Select a user to start chatting.</p>";
  }

  updateChatInputState();
}

async function handleChatUserClick(event) {
  const userButton = event.target.closest(".chat-user-btn");

  if (!userButton) {
    return;
  }

  selectedChatUserID = Number(userButton.dataset.userId);
  selectedChatUserOnline = userButton.dataset.online === "true";
  unreadChatUserIDs.delete(selectedChatUserID);
  oldestMessageID = null;

  chatTitle.textContent = `Chat with ${userButton.dataset.nickname}`;
  chatInput.value = "";
  updateChatInputState();

  await loadChatMessages(selectedChatUserID);
  await loadChatUsers();
}

async function loadChatMessages(userID) {
  chatMessages.innerHTML = "<p>Loading messages...</p>";

  try {
    const response = await fetch(`/api/chat/messages?user_id=${userID}`);
    const data = await readResponseJSON(response);

    if (!response.ok) {
      chatMessages.innerHTML = "<p>Failed to load messages.</p>";
      showMessage(data.error || "Failed to load messages", true);
      return;
    }

    renderChatMessages(data.messages);
    scrollChatToBottom();
  } catch (error) {
    chatMessages.innerHTML = "<p>Network error while loading messages.</p>";
    showMessage("Network error while loading messages", true);
  }
}

function renderChatMessages(messages) {
  chatMessages.innerHTML = "";
  displayedChatMessageIDs.clear();

  if (!messages || messages.length === 0) {
    chatMessages.innerHTML = "<p>No messages yet.</p>";
    oldestMessageID = null;
    return;
  }

  oldestMessageID = messages[0].id;
  appendOlderMessagesButton();

  messages.forEach((message) => {
    appendChatMessage(message);
  });
}

async function loadOlderChatMessages() {
  if (!selectedChatUserID || !oldestMessageID || isLoadingOlderMessages) {
    return;
  }

  isLoadingOlderMessages = true;

  const oldScrollHeight = chatMessages.scrollHeight;

  try {
    const response = await fetch(
      `/api/chat/messages?user_id=${selectedChatUserID}&before_id=${oldestMessageID}`
    );

    if (!response.ok) {
      isLoadingOlderMessages = false;
      showMessage("Failed to load older messages", true);
      return;
    }

    const data = await readResponseJSON(response);

    if (!data.messages || data.messages.length === 0) {
      const loadOlderButton = chatMessages.querySelector(".load-older-messages-btn");
      if (loadOlderButton) {
        loadOlderButton.textContent = "No earlier messages";
        loadOlderButton.disabled = true;
      }

      isLoadingOlderMessages = false;
      return;
    }

    const loadOlderButton = chatMessages.querySelector(".load-older-messages-btn");
    loadOlderButton?.remove();

    oldestMessageID = data.messages[0].id;
    const olderMessages = document.createDocumentFragment();

    data.messages.forEach((message) => {
      const messageID = Number(message.id);

      if (displayedChatMessageIDs.has(messageID)) {
        return;
      }

      displayedChatMessageIDs.add(messageID);
      olderMessages.appendChild(createChatMessageElement(message));
    });

    chatMessages.prepend(olderMessages);
    appendOlderMessagesButton();

    const newScrollHeight = chatMessages.scrollHeight;
    chatMessages.scrollTop = newScrollHeight - oldScrollHeight;
  } catch (error) {
    showMessage("Failed to load older messages", true);
  }

  isLoadingOlderMessages = false;
}

function handleChatMessagesScroll() {
  if (chatScrollTimeout) {
    clearTimeout(chatScrollTimeout);
  }

  chatScrollTimeout = setTimeout(() => {
    if (chatMessages.scrollTop <= 20) {
      loadOlderChatMessages();
    }
  }, 300);
}

function appendOlderMessagesButton() {
  if (!oldestMessageID || chatMessages.querySelector(".load-older-messages-btn")) {
    return;
  }

  const loadOlderButton = document.createElement("button");
  loadOlderButton.type = "button";
  loadOlderButton.className = "btn btn-sm load-older-messages-btn";
  loadOlderButton.textContent = "Load earlier messages";
  loadOlderButton.addEventListener("click", loadOlderChatMessages);

  chatMessages.prepend(loadOlderButton);
}

function createChatMessageElement(message) {
  const messageElement = document.createElement("article");
  messageElement.className = "chat-message";

  const senderID = Number(message.sender_id);
  const currentUserID = getCurrentUserID();

  if (senderID === currentUserID) {
    messageElement.classList.add("own-message");
  }

  messageElement.innerHTML = `
    <div class="chat-message-header">
      <strong>${escapeHTML(message.sender_nickname)}</strong>
      <span>${escapeHTML(message.created_at)}</span>
    </div>

    <p>${escapeHTML(message.content)}</p>
  `;

  return messageElement;
}

function appendChatMessage(message) {
  const messageID = Number(message.id);

  if (displayedChatMessageIDs.has(messageID)) {
    return false;
  }

  displayedChatMessageIDs.add(messageID);
  const messageElement = createChatMessageElement(message);
  chatMessages.appendChild(messageElement);
  observeRevealElement(messageElement);

  return true;
}

function connectChatSocket() {
  if (chatSocket && chatSocket.readyState !== WebSocket.CLOSED) {
    return;
  }

  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  chatSocket = new WebSocket(`${protocol}://${window.location.host}/ws/chat`);

  chatSocket.addEventListener("open", () => {
    loadChatUsers();
  });

  chatSocket.addEventListener("message", handleChatSocketMessage);

  chatSocket.addEventListener("close", () => {
    chatSocket = null;
  });

  chatSocket.addEventListener("error", () => {
    showMessage("Chat connection error", true);
  });
}

function handleChatSocketMessage(event) {
  let data;

  try {
    data = JSON.parse(event.data);
  } catch (error) {
    showMessage("Invalid chat message received", true);
    return;
  }

  switch (data.type) {
    case "private_message":
      handleIncomingPrivateMessage(data.message);
      break;
    case "presence":
      handleChatPresenceUpdate(data);
      break;
    case "error":
      showMessage(data.error || "Chat error", true);
      break;
  }
}

function handleChatPresenceUpdate(data) {
  const userID = Number(data.user_id);

  if (userID === selectedChatUserID) {
    selectedChatUserOnline = Boolean(data.online);
    updateChatInputState();
  }

  loadChatUsers();
}

function handleIncomingPrivateMessage(message) {
  const senderID = Number(message.sender_id);
  const receiverID = Number(message.receiver_id);
  const currentUserID = getCurrentUserID();

  const otherUserID = senderID === currentUserID ? receiverID : senderID;

  if (otherUserID && otherUserID !== selectedChatUserID) {
    unreadChatUserIDs.add(otherUserID);
  }

  loadChatUsers();

  if (!selectedChatUserID) {
    return;
  }

  if (!messageBelongsToSelectedChat(message)) {
    return;
  }

  unreadChatUserIDs.delete(selectedChatUserID);

  const emptyMessage = chatMessages.querySelector("p");

  if (emptyMessage && emptyMessage.textContent === "No messages yet.") {
    chatMessages.innerHTML = "";
  }

  const wasAdded = appendChatMessage(message);

  if (!wasAdded) {
    return;
  }

  if (!oldestMessageID) {
    oldestMessageID = message.id;
  }

  scrollChatToBottom();
}

function messageBelongsToSelectedChat(message) {
  const currentUserID = getCurrentUserID();
  const senderID = Number(message.sender_id);
  const receiverID = Number(message.receiver_id);

  return (
    (senderID === currentUserID && receiverID === selectedChatUserID) ||
    (senderID === selectedChatUserID && receiverID === currentUserID)
  );
}

function handleSendChatMessage(event) {
  event.preventDefault();

  if (!selectedChatUserID) {
    showMessage("Select a user first", true);
    return;
  }

  if (!selectedChatUserOnline) {
    showMessage("User is offline", true);
    return;
  }

  if (!chatSocket || chatSocket.readyState !== WebSocket.OPEN) {
    showMessage("Chat is not connected", true);
    return;
  }

  const content = chatInput.value.trim();

  if (content === "") {
    showMessage("Message cannot be empty", true);
    return;
  }

  chatSocket.send(JSON.stringify({
    type: "private_message",
    receiver_id: selectedChatUserID,
    content,
  }));

  chatInput.value = "";
}

function closeChatSocket() {
  if (chatSocket) {
    chatSocket.close();
    chatSocket = null;
  }
}

function clearChatState() {
  selectedChatUserID = null;
  selectedChatUserOnline = false;
  oldestMessageID = null;
  isLoadingOlderMessages = false;
  displayedChatMessageIDs.clear();
  unreadChatUserIDs.clear();

  chatTitle.textContent = "Select a user";
  chatUsersList.innerHTML = "";
  chatMessages.innerHTML = "<p>Select a user to start chatting.</p>";
  chatInput.value = "";
  updateChatInputState();
}

function scrollChatToBottom() {
  chatMessages.scrollTop = chatMessages.scrollHeight;
}

function getCurrentUserID() {
  return Number(currentUser?.id || currentUser?.ID || 0);
}

function updateChatInputState() {
  const sendButton = chatForm.querySelector("button");
  const canSend = Boolean(selectedChatUserID && selectedChatUserOnline);

  chatInput.disabled = !canSend;
  sendButton.disabled = !canSend;

  if (!selectedChatUserID) {
    chatInput.placeholder = "Select a user to start chatting.";
    return;
  }

  chatInput.placeholder = selectedChatUserOnline
    ? "Write a private message..."
    : "User is offline. You can view previous messages.";
}

function updateProfileSummary(user) {
  const headerAvatar = document.getElementById("header-avatar");

  if (headerAvatar) {
    headerAvatar.textContent = initials(user.nickname);
  }
}

// --- admin ----------------------------------------------------------------

function updateAdminVisibility(user) {
  const isAdmin = Boolean(user?.is_admin);

  adminOnlyElements.forEach((element) => {
    element.classList.toggle("hidden", !isAdmin);
  });

  if (!isAdmin && adminPanel) {
    adminPanel.classList.add("hidden");
  }
}

function handleAdminLinkClick() {
  // The anchor still scrolls to #admin-panel; we reveal and load it here.
  if (adminPanel) {
    adminPanel.classList.remove("hidden");
  }

  loadAdminOverview();
}

async function loadAdminOverview() {
  if (!adminPanel) {
    return;
  }

  adminUsersList.innerHTML = "<p>Loading...</p>";
  adminPostsList.innerHTML = "<p>Loading...</p>";
  adminCommentsList.innerHTML = "<p>Loading...</p>";

  try {
    const response = await fetch("/api/admin/overview");
    const data = await readResponseJSON(response);

    if (!response.ok) {
      showMessage(data.error || "Failed to load admin data", true);
      adminUsersList.innerHTML = "<p>Failed to load.</p>";
      adminPostsList.innerHTML = "";
      adminCommentsList.innerHTML = "";
      return;
    }

    renderAdminUsers(data.users);
    renderAdminPosts(data.posts);
    renderAdminComments(data.comments);
  } catch (error) {
    showMessage("Network error while loading admin data", true);
  }
}

function renderAdminUsers(users) {
  adminUsersList.innerHTML = "";

  if (!users || users.length === 0) {
    adminUsersList.innerHTML = '<p class="empty-state">No accounts.</p>';
    return;
  }

  users.forEach((user) => {
    const row = document.createElement("div");
    row.className = "admin-row";

    const adminBadge = user.is_admin ? '<span class="badge">admin</span>' : "";

    // Admin accounts (including your own) have no delete button, to avoid lockout.
    const deleteButton = user.is_admin
      ? ""
      : `<button class="delete-btn admin-delete-user-btn" type="button" data-user-id="${user.id}">Delete</button>`;

    row.innerHTML = `
      <div class="admin-row-main">
        <span class="admin-row-title">${escapeHTML(user.nickname)} ${adminBadge}</span>
        <span class="admin-row-meta">${escapeHTML(user.email)} · ${Number(user.post_count) || 0} posts · ${Number(user.comment_count) || 0} comments</span>
      </div>
      ${deleteButton}
    `;

    adminUsersList.appendChild(row);
  });
}

function renderAdminPosts(posts) {
  adminPostsList.innerHTML = "";

  if (!posts || posts.length === 0) {
    adminPostsList.innerHTML = '<p class="empty-state">No posts.</p>';
    return;
  }

  posts.forEach((post) => {
    const row = document.createElement("div");
    row.className = "admin-row";

    row.innerHTML = `
      <div class="admin-row-main">
        <span class="admin-row-title">${escapeHTML(post.title)}</span>
        <span class="admin-row-meta">by ${escapeHTML(post.author)} · ${Number(post.like_count) || 0} likes · ${Number(post.comment_count) || 0} comments · ${escapeHTML(post.created_at)}</span>
      </div>
      <button class="delete-btn admin-delete-post-btn" type="button" data-post-id="${post.id}">Delete</button>
    `;

    adminPostsList.appendChild(row);
  });
}

function renderAdminComments(comments) {
  adminCommentsList.innerHTML = "";

  if (!comments || comments.length === 0) {
    adminCommentsList.innerHTML = '<p class="empty-state">No comments.</p>';
    return;
  }

  comments.forEach((comment) => {
    const row = document.createElement("div");
    row.className = "admin-row";

    row.innerHTML = `
      <div class="admin-row-main">
        <span class="admin-row-text">${escapeHTML(comment.content)}</span>
        <span class="admin-row-meta">by ${escapeHTML(comment.author)} on "${escapeHTML(comment.post_title)}" · ${escapeHTML(comment.created_at)}</span>
      </div>
      <button class="delete-btn admin-delete-comment-btn" type="button" data-comment-id="${comment.id}">Delete</button>
    `;

    adminCommentsList.appendChild(row);
  });
}

function handleAdminPanelClick(event) {
  const userButton = event.target.closest(".admin-delete-user-btn");
  if (userButton) {
    deleteAdminUser(Number(userButton.dataset.userId));
    return;
  }

  const postButton = event.target.closest(".admin-delete-post-btn");
  if (postButton) {
    deleteAdminPost(Number(postButton.dataset.postId));
    return;
  }

  const commentButton = event.target.closest(".admin-delete-comment-btn");
  if (commentButton) {
    deleteAdminComment(Number(commentButton.dataset.commentId));
  }
}

async function deleteAdminUser(userID) {
  if (!userID) {
    return;
  }

  if (!confirm("Delete this account and all of its posts, comments, and likes?")) {
    return;
  }

  const result = await sendJSONRequest("/api/admin/users", "DELETE", {
    user_id: userID,
  });

  if (!result.ok) {
    showMessage(result.data.error || "Failed to delete account", true);
    return;
  }

  showMessage(result.data.message || "Account deleted successfully", false);

  await loadAdminOverview();
  await loadFeed();
  loadChatUsers();
}

async function deleteAdminPost(postID) {
  if (!postID) {
    return;
  }

  if (!confirm("Delete this post?")) {
    return;
  }

  const result = await sendJSONRequest("/api/admin/posts", "DELETE", {
    post_id: postID,
  });

  if (!result.ok) {
    showMessage(result.data.error || "Failed to delete post", true);
    return;
  }

  showMessage(result.data.message || "Post deleted successfully", false);

  if (selectedPostID === postID) {
    closeCommentsPanel();
  }

  await loadAdminOverview();
  await loadFeed();
}

async function deleteAdminComment(commentID) {
  if (!commentID) {
    return;
  }

  if (!confirm("Delete this comment?")) {
    return;
  }

  const result = await sendJSONRequest("/api/admin/comments", "DELETE", {
    comment_id: commentID,
  });

  if (!result.ok) {
    showMessage(result.data.error || "Failed to delete comment", true);
    return;
  }

  showMessage(result.data.message || "Comment deleted successfully", false);

  await loadAdminOverview();

  if (selectedPostID) {
    await loadComments(selectedPostID);
  }

  await loadFeed();
}

function setupTheme() {
  const savedTheme = localStorage.getItem("theme");

  if (savedTheme) {
    applyTheme(savedTheme);
  } else {
    updateThemeToggleLabels(getActiveTheme());
  }

  themeToggleButtons.forEach((button) => {
    button.addEventListener("click", () => {
      const nextTheme = getActiveTheme() === "dark" ? "light" : "dark";
      localStorage.setItem("theme", nextTheme);
      applyTheme(nextTheme);
    });
  });
}

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme;
  updateThemeToggleLabels(theme);
}

function getActiveTheme() {
  const selectedTheme = document.documentElement.dataset.theme;

  if (selectedTheme) {
    return selectedTheme;
  }

  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function updateThemeToggleLabels(theme) {
  themeToggleButtons.forEach((button) => {
    button.setAttribute("aria-label", `Switch to ${theme === "dark" ? "light" : "dark"} theme`);
  });
}

function setupMobileDrawer() {
  if (!mobileMenuButton || !mobileDrawer || !mobileDrawerClose) {
    return;
  }

  mobileMenuButton.addEventListener("click", openMobileDrawer);
  mobileDrawerClose.addEventListener("click", closeMobileDrawer);
  mobileDrawerBackdrop?.addEventListener("click", closeMobileDrawer);

  mobileDrawerLinks.forEach((link) => {
    link.addEventListener("click", closeMobileDrawer);
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      closeMobileDrawer();
    }
  });
  document.addEventListener("keydown", trapMobileDrawerFocus);
}

function openMobileDrawer() {
  lastFocusedElement = document.activeElement;
  mobileDrawer.classList.add("open");
  mobileDrawer.removeAttribute("aria-hidden");
  mobileDrawerBackdrop?.classList.add("backdrop-visible");
  mobileMenuButton.setAttribute("aria-expanded", "true");
  document.body.style.overflow = "hidden";
  mobileDrawer.querySelector("a, button")?.focus();
}

function closeMobileDrawer() {
  mobileDrawer.classList.remove("open");
  mobileDrawer.setAttribute("aria-hidden", "true");
  mobileDrawerBackdrop?.classList.remove("backdrop-visible");
  mobileMenuButton.setAttribute("aria-expanded", "false");
  document.body.style.overflow = "";

  if (lastFocusedElement instanceof HTMLElement) {
    lastFocusedElement.focus();
  }
}

function trapMobileDrawerFocus(event) {
  if (event.key !== "Tab" || !mobileDrawer.classList.contains("open")) {
    return;
  }

  const focusableElements = mobileDrawer.querySelectorAll(
    'a[href], button:not([disabled]), input:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
  );

  if (focusableElements.length === 0) {
    return;
  }

  const firstElement = focusableElements[0];
  const lastElement = focusableElements[focusableElements.length - 1];

  if (event.shiftKey && document.activeElement === firstElement) {
    event.preventDefault();
    lastElement.focus();
    return;
  }

  if (!event.shiftKey && document.activeElement === lastElement) {
    event.preventDefault();
    firstElement.focus();
  }
}

function setupRevealObserver() {
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    return;
  }

  revealObserver = new IntersectionObserver((entries) => {
    entries.forEach((entry) => {
      if (entry.isIntersecting) {
        entry.target.classList.add("revealed");
        revealObserver.unobserve(entry.target);
      }
    });
  }, {
    threshold: 0.1,
  });
}

function observeRevealElement(element) {
  if (!revealObserver) {
    element.classList.add("revealed");
    return;
  }

  revealObserver.observe(element);
}
