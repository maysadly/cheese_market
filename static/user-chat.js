let currentChatID = localStorage.getItem("currentChatID") || null;
const socket = new WebSocket('ws://localhost:8080/ws');

document.addEventListener("DOMContentLoaded", () => {
    checkActiveChat();
});
async function checkActiveChat() {
    console.log("Проверяем активный чат...");

    // 1. Проверяем localStorage
    const storedChatID = localStorage.getItem("currentChatID");
    if (storedChatID) {
        console.log("Найден чат в localStorage:", storedChatID);
        updateChatButton(storedChatID);
        updateUIForActiveChat()
        return;
    }

    // 2. Запрашиваем у сервера
    const response = await fetch("/api/get-active-chat");
    const data = await response.json();
    
    console.log("Ответ API (active-chat):", data);
    
    if (data.active) {
        console.log("Сервер вернул активный чат:", data.chat_id);
        localStorage.setItem("currentChatID", data.chat_id);
        updateChatButton(data.chat_id);
    } else {
        console.log("Активного чата нет.");
    }
}


// Проверка, есть ли активный чат
async function createNewChat() {
    console.log("Нажата кнопка 'Create New Chat'");
    
    const userID = await getUserIDFromToken();
    console.log("UserID:", userID);
    
    if (!userID) {
        console.error("Ошибка: user_id отсутствует!");
        return;
    }

    const chatData = JSON.stringify({
        type: "create_chat",
        user_id: userID
    });

    console.log("Отправка запроса на создание чата:", chatData);
    socket.send(chatData);
}


// Обновление UI при наличии активного чата
function updateUIForActiveChat() {
    document.getElementById('newChatBtn').style.display = 'none';
    document.getElementById('activeChatLink').style.display = 'block';
}

// Обновление UI, если чата нет
function updateUIForNewChat() {
    document.getElementById('newChatBtn').style.display = 'block';
    document.getElementById('activeChatLink').style.display = 'none';
}

// Функция получения токена
async function getTokenFromServer() {
    const response = await fetch("/login", { method: "POST", credentials: "include" });

    let token = response.headers.get("Authorization")?.split(" ")[1];
    if (!token) {
        token = getCookie("token");
    }

    return token || null;
}

async function getUserIDFromToken() {
    const token = await getTokenFromServer();
    if (!token) {
        console.error("Ошибка: токен не получен!");
        return null;
    }

    try {
        const payload = JSON.parse(atob(token.split(".")[1])); // Декодируем payload
        console.log("user_id из токена:", payload.user_id); // Логируем user_id
        return payload.user_id;
    } catch (error) {
        console.error("Ошибка декодирования токена:", error);
        return null;
    }
}

async function createNewChat() {
    console.log("Создание нового чата...");
    const userID = await getUserIDFromToken();
    console.log("userID:", userID);
    
    if (!userID) {
        console.error("Ошибка: user_id отсутствует!");
        return;
    }

    socket.send(JSON.stringify({
        type: "create_chat",
        user_id: userID
    }));

    console.log("Запрос на создание чата отправлен!");
}

socket.onmessage = function (event) {
    const data = JSON.parse(event.data);
    console.log("WebSocket получил сообщение:", data);

    if (data.type === "chat_created") {
        console.log("Чат создан с ID:", data.chat_id);

        localStorage.setItem("currentChatID", data.chat_id);
        console.log("Сохранён в localStorage:", localStorage.getItem("currentChatID"));

        updateChatButton(data.chat_id);
    }
};
function updateChatButton(chatID) {
    console.log("Обновляем кнопку для чата:", chatID);
}


async function loadChatHistory() {
    if (!currentChatID) {
        console.error("Ошибка: currentChatID отсутствует!");
        return;
    }

    try {
        const response = await fetch(`/api/chat-history?chat_id=${currentChatID}`);
        const data = await response.json();

        if (Array.isArray(data)) {
            data.forEach(msg => addMessageToUI(msg.sender, msg.content));
        } else {
            console.error("Ошибка: Ожидался массив, получено:", data);
        }
    } catch (error) {
        console.error("Ошибка загрузки истории чата:", error);
    }
}

function addMessageToUI(sender, message) {
    const messagesDiv = document.getElementById('messages');
    const messageDiv = document.createElement('div');
    messageDiv.className = `message ${sender}`;
    messageDiv.innerHTML = `<strong>${sender}:</strong> ${message}`;
    messagesDiv.appendChild(messageDiv);
}

document.getElementById('activeChatLink').addEventListener('click', function (event) {
    event.preventDefault();
    document.getElementById('chatWindow').style.display = 'block';
    document.getElementById('chatControl').style.display = 'none';
    document.getElementById('chatID').textContent = currentChatID;
    loadChatHistory();
});
function getCookie(name) {
    const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'));
    return match ? match[2] : null;
}
function sendMessage() {
    loadChatHistory
    const input = document.getElementById('messageInput');
    if (!input.value.trim()) return;

    const messageData = {
        type: "send_message",
        chat_id: currentChatID,
        sender: "user",
        content: input.value.trim()
    };

    socket.send(JSON.stringify(messageData));
    addMessageToUI("user", input.value.trim());
    input.value = '';
}
document.getElementById("logout-button").addEventListener("click", () => {
    localStorage.removeItem("currentChatID"); 
    console.log("Чат удалён из localStorage");
});
