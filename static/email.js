document.addEventListener('DOMContentLoaded', function () {
    const form = document.getElementById('emailForm');
    form.addEventListener('submit', async (event) => {
        event.preventDefault();

        const to = document.getElementById('to').value;
        const subject = document.getElementById('subject').value;
        const message = document.getElementById('message').value;
        const fileInput = document.getElementById('file');
        const file = fileInput.files[0];
        let fileContent = null;

        if (file) {
            fileContent = await readFile(file);
        }

        const payload = {
            to,
            subject,
            body: message,
            file: file
                ? {
                    filename: file.name,
                    content: fileContent
                }
                : {}
        };

        fetch('http://localhost:8081/send_email', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        })
            .then(async (response) => {
                let result;
                try {
                    result = await response.json();
                } catch (e) {
                    throw new Error(`Invalid JSON response: ${response.status} - ${response.statusText}`);
                }

                if (response.ok) {
                    alert(result.status || "Email sent successfully!");
                } else {
                    alert(result.error || "An error occurred");
                }
            })
            .catch(error => {
                alert(`Request failed: ${error.message}`);
            });

    });

    function readFile(file) {
        return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = () => resolve(reader.result);
            reader.onerror = reject;
            reader.readAsDataURL(file);
        });
    }
});


document.addEventListener('DOMContentLoaded', async function () {
    const selectElement = document.getElementById('to');

    try {
        const response = await fetch('/get_users_email_list');
        if (response.ok) {
            const emailList = await response.json();

            emailList.forEach(email => {
                const option = document.createElement('option');
                option.value = email;
                option.textContent = email;
                selectElement.appendChild(option);
            });
        } else {
            console.error("Failed to load email list");
        }
    } catch (error) {
        console.error("Request failed", error);
    }
});
