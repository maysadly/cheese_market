
document.addEventListener('DOMContentLoaded', function () {
    const form = document.getElementById('emailForm');
    form.addEventListener('submit', async (event) => {
        event.preventDefault();

        const to = document.getElementById('to').value;
        const message = document.getElementById('message').value;

        const payload = { to, message };

        try {
            const response = await fetch('/send-email', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });

            const text = await response.text();
            try {
                const result = JSON.parse(text);
                if (response.ok) {
                    alert(result.message);
                    form.reset();
                } else {
                    alert(`Error: ${result.message}`);
                }
            } catch (error) {
                alert(`Response was not valid JSON: ${text}`);
            }
        } catch (error) {
            alert(`Request failed: ${error}`);
        }
    });
});

