async function fetchProducts() {
    const response = await fetch("http://localhost:8080/products");
    const products = await response.json();
    const productsList = document.getElementById("products");
    productsList.innerHTML = "";

    products.forEach((product) => {
        const item = document.createElement("li");
        item.textContent = `${product.name} - $${product.price}`;

        const deleteButton = document.createElement("button");
        deleteButton.textContent = "x";
        deleteButton.classList = "delete";

        deleteButton.onclick = () => deleteProduct(product.id);

        item.appendChild(deleteButton);
        productsList.appendChild(item);
    });
}

async function addProduct(event) {
    event.preventDefault();
    const form = document.getElementById("form")
    const name = document.getElementById("name").value;
    const price = document.getElementById("price").value;

    const response = await fetch("http://localhost:8080/products", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, price: parseFloat(price) }),
    });

    if (response.ok) {
        await fetchProducts();
    } else {
        alert("Failed to add product.");
    }

    document.getElementById("name").value = ""
    document.getElementById("price").value = ""
}

async function deleteProduct(id) {
    const response = await fetch("http://localhost:8080/products", {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id }),
    });

    if (response.ok) {
        await fetchProducts();
    } else {
        alert("Failed to delete product.");
    }
}


window.onload = fetchProducts;