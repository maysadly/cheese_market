let activeChats = [];
let currentChatID = null;
const socket = new WebSocket('ws://localhost:8080/ws');

// Load active chats on startup
window.onload = function () {
    loadActiveChats();
};

// Fetch the list of active chats
async function loadActiveChats() {
    try {
        const response = await fetch('/api/active-chats');
        const data = await response.json();
        activeChats = data;
        renderActiveChats();
    } catch (error) {
        console.error("Error loading active chats:", error);
    }
}

// Display the list of active chats
function renderActiveChats() {
    const chatListDiv = document.getElementById('activeChats');
    chatListDiv.innerHTML = '';

    activeChats.forEach(chat => {
        const chatItem = document.createElement('div');
        chatItem.className = 'chat-item';
        chatItem.textContent = `Chat #${chat.chat_id}`;
        chatItem.onclick = () => openChat(chat.chat_id);
        chatListDiv.appendChild(chatItem);
    });
}

// Open a chat
function openChat(chatID) {
    currentChatID = chatID;
    document.getElementById('chatBox').style.display = 'block';
    document.getElementById('chatMessages').innerHTML = '';
    loadChatHistory();
}

// Send a message as an admin
function sendMessage() {
    const input = document.getElementById('chatInput');
    if (!input.value.trim()) return;

    const messageData = {
        type: "send_message",
        chat_id: currentChatID,
        sender: "admin",
        content: input.value.trim()
    };

    socket.send(JSON.stringify(messageData));
    input.value = '';

    // Load updated chat history after sending
    setTimeout(loadChatHistory, 300);
}

// Handle incoming WebSocket messages
socket.onmessage = function (event) {
    console.log("Received WebSocket message:", event.data);
    const data = JSON.parse(event.data);

    switch (data.type) {
        case 'chat_created':
            console.log("Chat created with ID:", data.chat_id);
            currentChatID = data.chat_id;
            document.getElementById('chatID').textContent = currentChatID;
            document.getElementById('chatWindow').style.display = 'block';
            document.getElementById('chatControl').style.display = 'none';
            break;

        case 'new_message':
            console.log("New message received:", data);
            if (data.chat_id === currentChatID) {
                loadChatHistory();
            }
            break;
    }
};

// Load chat history
async function loadChatHistory() {
    if (!currentChatID) {
        console.error("Error: currentChatID is missing!");
        return;
    }

    try {
        const response = await fetch(`/api/chat-history?chat_id=${currentChatID}`);
        const data = await response.json();

        if (Array.isArray(data)) {
            const messagesDiv = document.getElementById('chatMessages');
            messagesDiv.innerHTML = '';
            data.forEach(msg => addMessageToUI(msg.sender, msg.content));
        } else {
            console.error("Error: Expected an array, received:", data);
        }
    } catch (error) {
        console.error("Error loading chat history:", error);
    }
}

// Add messages to the UI
function addMessageToUI(sender, message) {
    const messagesDiv = document.getElementById('chatMessages');

    const messageDiv = document.createElement('div');
    messageDiv.className = `message ${sender}`;
    messageDiv.innerHTML = `<strong>${sender}:</strong> ${message}`;
    messagesDiv.appendChild(messageDiv);
}

// Close chat
async function closeChat() {
    if (!currentChatID) {
        console.error("Error: No active chat to close.");
        return;
    }

    const closeMessage = {
        type: "close_chat",
        chat_id: currentChatID
    };

    socket.send(JSON.stringify(closeMessage));
    console.log("Sending request to close chat...");

    // Clear local storage
    localStorage.removeItem("currentChatID");

    // Remove chat from active list
    activeChats = activeChats.filter(chat => chat.chat_id !== currentChatID);
    renderActiveChats();

    // Hide chat window
    document.getElementById("chatBox").style.display = "none";

    // Reset current chat
    currentChatID = null;

    // Refresh the page
    setTimeout(() => location.reload(), 500);

}
