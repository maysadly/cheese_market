let currentChatID = localStorage.getItem("currentChatID") || null;
const socket = new WebSocket('ws://localhost:8080/ws');

socket.onopen = function () {
    console.log("✅ WebSocket connection established.");

    if (currentChatID) {
        sendWebSocketMessage({ type: "check_chat", chat_id: currentChatID });
    } else {
        showCreateChatButton();
    }
};

socket.onerror = function (event) {
    console.error("❌ WebSocket error:", event);
};

socket.onclose = function () {
    console.warn("⚠️ WebSocket connection closed.");
};

function sendWebSocketMessage(data) {
    if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify(data));
    } else {
        console.warn("WebSocket is not ready, message not sent:", data);
    }
}

document.addEventListener("DOMContentLoaded", () => {
    checkActiveChat();
});

window.onload = function () {
    console.log("Checking active chat...");
    const chatID = localStorage.getItem("currentChatID");

    if (chatID) {
        console.log("Chat found in localStorage:", chatID);
        updateUIForActiveChat(chatID);
    } else {
        console.log("No active chat, showing create button.");
        updateUIForNewChat();
    }
};

async function checkActiveChat() {
    console.log("Checking active chat...");

    const storedChatID = localStorage.getItem("currentChatID");
    if (storedChatID) {
        console.log("Chat found in localStorage:", storedChatID);
        updateUIForActiveChat(storedChatID);
        return;
    }

    try {
        const response = await fetch("/api/get-active-chat");
        const data = await response.json();

        console.log("API response (active-chat):", data);

        if (data.active && data.chat_id) {
            console.log("Server returned active chat:", data.chat_id);
            localStorage.setItem("currentChatID", data.chat_id);
            updateUIForActiveChat(data.chat_id);
        } else {
            console.log("No active chat.");
            updateUIForNewChat();
        }
    } catch (error) {
        console.error("Error fetching active chat:", error);
    }
}

async function createNewChat() {
    console.log("Create New Chat button clicked");

    const userID = await getUserIDFromToken();
    if (!userID) {
        console.error("Error: user_id is missing!");
        return;
    }

    const chatData = { type: "create_chat", user_id: userID };
    sendWebSocketMessage(chatData);
}

function updateUIForActiveChat(chatID) {
    document.getElementById("newChatBtn").style.display = "none";
    document.getElementById("activeChatLink").style.display = "block";
    document.getElementById("activeChatLink").setAttribute("data-chat-id", chatID);
}

function updateUIForNewChat() {
    document.getElementById("newChatBtn").style.display = "block";
    document.getElementById("activeChatLink").style.display = "none";
}



async function loadChatHistory(chatID) {
    if (!chatID) {
        console.error("Error: chatID is missing!");
        return;
    }

    try {
        const response = await fetch(`/api/chat-history?chat_id=${chatID}`);
        const data = await response.json();

        if (Array.isArray(data)) {
            data.forEach(msg => addMessageToUI(msg.sender, msg.content));
        } else {
            console.error("Error: Expected an array, received:", data);
        }
    } catch (error) {
        console.error("Error loading chat history:", error);
    }
}

function sendMessage() {
    const input = document.getElementById('messageInput');
    if (!input.value.trim()) return;

    const messageData = {
        type: "send_message",
        chat_id: localStorage.getItem("currentChatID"),
        sender: "user",
        content: input.value.trim()
    };

    if (!messageData.chat_id) {
        console.error("Error: chat_id is missing! Message not sent.");
        return;
    }

    socket.send(JSON.stringify(messageData)); 
    input.value = ''; // Очищаем поле ввода
}

socket.onmessage = function (event) {
    const data = JSON.parse(event.data);
    console.log("WebSocket received message:", data);

    if (data.type === "chat_status") {
        if (data.exists) {
            console.log("Chat found, loading history...");
            loadChatHistory(data.chat_id);
        } else {
            console.log("Chat not found, showing create button.");
            localStorage.removeItem("currentChatID");
            updateUIForNewChat();
        }
    }

    if (data.type === "chat_created") {
        console.log("Chat created:", data.chat_id);
        localStorage.setItem("currentChatID", data.chat_id);
        updateUIForActiveChat(data.chat_id);
    }

    if (data.type === "chat_closed") {
        const closedChatID = data.chat_id;
        document.getElementById("newChatBtn").style.display = "block";
        if (localStorage.getItem("currentChatID") === closedChatID) {
            console.log("Chat closed by admin. Clearing data...");
            localStorage.removeItem("currentChatID");
            document.getElementById("chatWindow").style.display = "none";
            document.getElementById("activeChatLink").style.display = "none";
            
            alert("Chat closed by administrator.");
        }
    }

    if (data.type === "new_message") {
        if (document.querySelector(`#messages .message[data-id="${data.message_id}"]`)) {
            console.warn("Duplicate message detected, skipping:", data.message_id);
            return;
        }

        addMessageToUI(data.sender, data.content, data.message_id);
    }
};

function addMessageToUI(sender, message, messageID = null) {
    const messagesDiv = document.getElementById("messages");
    const messageDiv = document.createElement("div");

    messageDiv.className = `message ${sender}`;
    messageDiv.innerHTML = `<strong>${sender}:</strong> ${message}`;
    if (messageID) messageDiv.setAttribute("data-id", messageID);

    messagesDiv.appendChild(messageDiv);
}


async function getUserIDFromToken() {
    try {
        const response = await fetch("/login", { method: "POST", credentials: "include" });
        const token = response.headers.get("Authorization")?.split(" ")[1] || getCookie("token");
        if (!token) throw new Error("Token not found!");

        const payload = JSON.parse(atob(token.split(".")[1]));
        console.log("user_id from token:", payload.user_id);
        return payload.user_id;
    } catch (error) {
        console.error("Error getting user_id:", error);
        return null;
    }
}

function getCookie(name) {
    const match = document.cookie.match(new RegExp(`(^| )${name}=([^;]+)`));
    return match ? match[2] : null;
}
document.getElementById("activeChatLink").addEventListener("click", (event) => {
    event.preventDefault();
    
    const chatID = localStorage.getItem("currentChatID");
    if (!chatID) {
        console.error("Ошибка: Чат отсутствует в localStorage!");
        return;
    }

    console.log("Переход к активному чату:", chatID);
    openChat(chatID);
});
function openChat(chatID) {
    document.getElementById("chatControl").style.display = "none"; // Скрываем управление
    document.getElementById("chatWindow").style.display = "block"; // Показываем окно чата
    document.getElementById("chatID").textContent = chatID; // Отображаем номер чата

    console.log("Загружаем историю сообщений для чата:", chatID);
    loadChatHistory(chatID);
}
document.getElementById("logout-button").addEventListener("click", () => {
    localStorage.removeItem("currentChatID");
    console.log("Chat removed from localStorage");
});
