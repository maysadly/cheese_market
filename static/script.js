let currentPage = 1;
let pageSize = 5; // Number of products per page
let sortBy = "name"; // Default sorting field
let sortOrder = "asc"; // Default sorting order (ascending)
let category = ""; // Default category filter
let cart = [];

function addToCart(product) {
    const existingProduct = cart.find((item) => item.id === product.id);
    if (existingProduct) {
        existingProduct.quantity += 1;
    } else {
        cart.push({ ...product, quantity: 1 });
    }
    renderCart();
}

function renderCart() {
    const cartList = document.getElementById("cart");
    cartList.innerHTML = "";

    cart.forEach((item) => {
        const cartItem = document.createElement("li");
        cartItem.textContent = `${item.name} - $${item.price} x ${item.quantity}`;

        const removeButton = document.createElement("button");
        removeButton.textContent = "Delete";
        removeButton.onclick = () => {
            cart = cart.filter((cartItem) => cartItem.id !== item.id);
            renderCart();
        };

        cartItem.appendChild(removeButton);
        cartList.appendChild(cartItem);
    });

    const total = cart.reduce((sum, item) => sum + item.price * item.quantity, 0);
    const totalDisplay = document.createElement("p");
    totalDisplay.textContent = `Total: $${total.toFixed(2)}`;
    cartList.appendChild(totalDisplay);
}

async function fetchProducts() {
    const response = await fetch(`http://localhost:8080/products?page=${currentPage}&pageSize=${pageSize}&sortBy=${sortBy}&sortOrder=${sortOrder}&category=${category}`);
    const { products, total } = await response.json(); // Assuming the server returns `products` and `total`
    const productsList = document.getElementById("products");
    productsList.innerHTML = "";

    products.forEach((product) => {
        const item = document.createElement("li");
        item.textContent = `${product.name} - $${product.price}`;

        const deleteButton = document.createElement("button");
        deleteButton.textContent = "×";
        deleteButton.classList = "delete";

        const updateButton = document.createElement("button");
        updateButton.textContent = "update";
        updateButton.classList = "update";

        const AddToCartButton = document.createElement("button");
        AddToCartButton.textContent = "Add";
        AddToCartButton.classList = "add";

        AddToCartButton.onclick = () => addToCart(product);

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
                item.appendChild(AddToCartButton);
            };

            item.innerHTML = "";
            item.appendChild(inputName);
            item.appendChild(inputPrice);
            item.appendChild(saveButton);
        };

        item.appendChild(updateButton);
        item.appendChild(deleteButton);
        item.appendChild(AddToCartButton);
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
    prevButton.textContent = "←";
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
    nextButton.textContent = "→";
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
    const categoryAdd = document.getElementById("categoryAdd").value;

    if (isNaN(price)) {
        alert("Please enter a valid price.");
        return;
    }

    const response = await fetch("http://localhost:8080/products", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, price, category: categoryAdd }),
    });

    if (response.ok) {
        await fetchProducts(); // Refresh the product list after adding
    } else {
        alert("Failed to add product.");
    }

    // Clear input fields after successful submission
    document.getElementById("name").value = "";
    document.getElementById("price").value = "";
    document.getElementById("categoryAdd").value = "";
}


async function deleteProduct(id) {
    const response = await fetch(`http://localhost:8080/products/${id}`, {
        method: "DELETE",
    });

    if (response.ok) {
        await fetchProducts();
    } else {
        const errorMessage = await response.text();
        alert(`Failed to delete product. ${errorMessage}`);
    }
}

async function updateProduct(id, updatedName, updatedPrice) {
    const response = await fetch(`http://localhost:8080/products/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: updatedName, price: updatedPrice }),
    });

    if (response.ok) {
        await fetchProducts();
    } else {
        const errorMessage = await response.text();
        alert(`Failed to update product. ${errorMessage}`);
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
        
        if (product && product.products && product.products.length > 0) {
            const p = product.products[0];  // Assuming you're getting an array of products
            resultDiv.innerHTML = `
                <p><strong>ID:</strong> ${p.id || p._id}</p> <!-- Use p.id or p._id depending on your server response -->
                <p><strong>Name:</strong> ${p.name}</p>
                <p><strong>Price:</strong> $${p.price}</p>
            `;
        } else {
            resultDiv.innerHTML = "<p class='result-error'>Product not found.</p>";
        }
    } catch (error) {
        resultDiv.innerHTML = `<p class='result-error'>Error: ${error.message}</p>`;
    }
}

document.getElementById("category").onchange = (event) => {
    category = event.target.value;
    fetchProducts();
};
function checkout() {
    if (cart.length === 0) {
        alert("Your cart is empty! Please add items to your cart before checking out.");
        return;
    }

    fetch("http://localhost:8080/cart", {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify(cart),
    })
        .then((response) => {
            if (response.ok) {
                alert("Order is created successfully!");
                cart = [];
                renderCart();
            } else {
                alert("Error");
            }
        })
        .catch((error) => console.error("Error:", error));
}


window.onload = fetchProducts;
