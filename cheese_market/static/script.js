let currentPage = 1;
let pageSize = 5; // Number of products per page
let sortBy = "name"; // Default sorting field
let sortOrder = "asc"; // Default sorting order (ascending)

async function fetchProducts() {
    const response = await fetch(`http://localhost:8080/products?page=${currentPage}&pageSize=${pageSize}&sortBy=${sortBy}&sortOrder=${sortOrder}`);
    const { products, total } = await response.json(); // Assuming the server returns `products` and `total`
    const productsList = document.getElementById("products");
    productsList.innerHTML = "";

    products.forEach((product) => {
        const item = document.createElement("li");
        item.textContent = `${product.name} - $${product.price}`;

        const deleteButton = document.createElement("button");
        deleteButton.textContent = "x";
        deleteButton.classList = "delete";

        const updateButton = document.createElement("button");
        updateButton.textContent = "update";
        updateButton.classList = "update";

        deleteButton.onclick = () => deleteProduct(product.id);

        updateButton.onclick = () => {
            const inputName = document.createElement("input");
            inputName.type = "text";
            inputName.value = product.name;
            inputName.classList = "input_update";

            const inputPrice = document.createElement("input");
            inputPrice.type = "number";
            inputPrice.value = product.price;
            inputPrice.classList = "input_update";

            const saveButton = document.createElement("button");
            saveButton.textContent = "save";
            saveButton.classList = "save";

            saveButton.onclick = () => {
                product.name = inputName.value;
                product.price = parseFloat(inputPrice.value);
                updateProduct(product.id, product.name, product.price);
                item.textContent = `${product.name} - $${product.price}`;
                item.appendChild(updateButton);
                item.appendChild(deleteButton);
            };

            item.innerHTML = "";
            item.appendChild(inputName);
            item.appendChild(inputPrice);
            item.appendChild(saveButton);
        };

        item.appendChild(updateButton);
        item.appendChild(deleteButton);
        productsList.appendChild(item);
    });

    renderPagination(total);
}

function renderPagination(total) {
    const totalPages = Math.ceil(total / pageSize);
    const paginationContainer = document.getElementById("pagination");
    paginationContainer.innerHTML = ""; // Clear previous buttons

    // Create previous button
    const prevButton = document.createElement("button");
    prevButton.textContent = "Previous";
    prevButton.disabled = currentPage === 1; // Disable on first page
    prevButton.onclick = () => {
        if (currentPage > 1) {
            currentPage--;
            fetchProducts();
        }
    };
    paginationContainer.appendChild(prevButton);

    // Create page buttons
    for (let i = 1; i <= totalPages; i++) {
        const pageButton = document.createElement("button");
        pageButton.textContent = i;
        pageButton.classList.add("page-button");
        if (i === currentPage) {
            pageButton.classList.add("active"); // Highlight current page
        }
        pageButton.onclick = () => {
            currentPage = i;
            fetchProducts();
        };
        paginationContainer.appendChild(pageButton);
    }

    // Create next button
    const nextButton = document.createElement("button");
    nextButton.textContent = "Next";
    nextButton.disabled = currentPage === totalPages; // Disable on last page
    nextButton.onclick = () => {
        if (currentPage < totalPages) {
            currentPage++;
            fetchProducts();
        }
    };
    paginationContainer.appendChild(nextButton);
}


function applySorting(newSortBy) {
    if (sortBy === newSortBy) {
        sortOrder = sortOrder === "asc" ? "desc" : "asc";
    } else {
        sortBy = newSortBy;
        sortOrder = "asc";
    }
    fetchProducts();
}

// Event listeners for sorting buttons
document.getElementById("sortByName").onclick = () => applySorting("name");
document.getElementById("sortByPrice").onclick = () => applySorting("price");

// Add product function
async function addProduct(event) {
    event.preventDefault();
    const form = document.getElementById("form");
    const name = document.getElementById("name").value;
    const price = parseFloat(document.getElementById("price").value); // Convert price to float

    if (isNaN(price)) {
        alert("Please enter a valid price.");
        return;
    }

    const response = await fetch("http://localhost:8080/products", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, price }), // Send the price as a float
    });

    if (response.ok) {
        await fetchProducts(); // Refresh the product list after adding
    } else {
        alert("Failed to add product.");
    }

    // Clear input fields after successful submission
    document.getElementById("name").value = "";
    document.getElementById("price").value = "";
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

async function updateProduct(id, updatedName, updatedPrice) {
    const response = await fetch("http://localhost:8080/products", {
        method: "PUT", 
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id, name: updatedName, price: updatedPrice }),
    });

    if (response.ok) {
        await fetchProducts(); 
    } else {
        alert(response.statusText);
    }
}
async function searchProduct() {
    const productId = document.getElementById("productId").value.trim();
    const resultDiv = document.getElementById("result");
    resultDiv.innerHTML = ""; // Clear previous results

    if (!productId) {
        resultDiv.innerHTML = "<p class='result-error'>Please enter a Product ID.</p>";
        return;
    }

    try {
        const response = await fetch(`/products?id=${productId}`);
        if (!response.ok) {
            if (response.status === 404) {
                resultDiv.innerHTML = "<p class='result-error'>Product not found.</p>";
            } else {
                resultDiv.innerHTML = "<p class='result-error'>Failed to fetch product details.</p>";
            }
            return;
        }

        const product = await response.json();
        resultDiv.innerHTML = `
            <p><strong>ID:</strong> ${product.id}</p>
            <p><strong>Name:</strong> ${product.name}</p>
            <p><strong>Price:</strong> $${product.price}</p>
        `;
    } catch (error) {
        resultDiv.innerHTML = `<p class='result-error'>Error: ${error.message}</p>`;
    }
}
window.onload = fetchProducts;