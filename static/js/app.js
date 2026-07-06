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

let selectedPostID = null;

let currentUser = null;
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

closeCommentsBtn.addEventListener("click", closeCommentsPanel);
createCommentForm.addEventListener("submit", handleCreateComment);
commentsList.addEventListener("click", handleCommentsListClick);

chatUsersList.addEventListener("click", handleChatUserClick);
chatForm.addEventListener("submit", handleSendChatMessage);
chatMessages.addEventListener("scroll", handleChatMessagesScroll);

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
    postElement.className = "post-card reveal";

    const categories = Array.isArray(post.categories) ? post.categories : [];
    const primaryCategory = categories[0] || "general";

    postElement.innerHTML = `
      <div class="post-header">
        <div class="post-author">
          <span class="avatar">${escapeHTML(initials(post.author))}</span>
          <div>
            <strong>${escapeHTML(post.author)}</strong>
            <span class="mono">@${escapeHTML(post.author)}</span>
          </div>
        </div>

        <div class="post-header-meta">
          <span class="tag">${escapeHTML(primaryCategory)}</span>
          <span class="mono">${escapeHTML(post.created_at)}</span>
        </div>
      </div>

      <h4>${escapeHTML(post.title)}</h4>
      <p class="post-preview">${escapeHTML(post.content)}</p>

      <div class="post-categories">
        ${categories.map((category) => `<span>${escapeHTML(category)}</span>`).join("")}
      </div>

      <div class="post-meta">
        <span class="mono">${Number(post.like_count) || 0} votes</span>
        <span class="mono">${Number(post.comment_count) || 0} comments</span>
        <button class="icon-action" type="button" aria-label="Share post">S</button>
        <button class="icon-action" type="button" aria-label="Save post">B</button>
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
    observeRevealElement(postElement);
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
    commentElement.className = "comment-card reveal";

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

  guestView.classList.add("hidden");
  userView.classList.remove("hidden");

  clearMessage();

  loadPosts();
  loadChatUsers();
  connectChatSocket();
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
      <span>${escapeHTML(user.nickname)}</span>

      <span class="chat-user-meta">
        ${hasUnread ? '<strong class="unread-badge">New</strong>' : ""}
        <small class="${statusClass}">${statusText}</small>
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
      isLoadingOlderMessages = false;
      return;
    }

    oldestMessageID = data.messages[0].id;

    for (let index = data.messages.length - 1; index >= 0; index--) {
      const message = data.messages[index];
      const messageID = Number(message.id);

      if (displayedChatMessageIDs.has(messageID)) {
        continue;
      }

      displayedChatMessageIDs.add(messageID);
      chatMessages.prepend(createChatMessageElement(message));
    }

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
  const profileName = document.querySelector(".profile-mini .profile-row strong");
  const profileHandle = document.querySelector(".profile-mini .profile-row .mono");
  const profileAvatar = document.querySelector(".profile-mini .avatar");

  if (!profileName || !profileHandle || !profileAvatar) {
    return;
  }

  profileName.textContent = user.nickname;
  profileHandle.textContent = `@${user.nickname}`;
  profileAvatar.textContent = initials(user.nickname);
}

function setupTheme() {
  const savedTheme = localStorage.getItem("theme");
  const preferredTheme = window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";

  applyTheme(savedTheme || preferredTheme);

  themeToggleButtons.forEach((button) => {
    button.addEventListener("click", () => {
      const nextTheme = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
      localStorage.setItem("theme", nextTheme);
      applyTheme(nextTheme);
    });
  });
}

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme;

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

  mobileDrawerLinks.forEach((link) => {
    link.addEventListener("click", closeMobileDrawer);
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      closeMobileDrawer();
    }
  });
}

function openMobileDrawer() {
  mobileDrawer.classList.add("open");
  mobileMenuButton.setAttribute("aria-expanded", "true");
}

function closeMobileDrawer() {
  mobileDrawer.classList.remove("open");
  mobileMenuButton.setAttribute("aria-expanded", "false");
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
