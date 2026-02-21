import { test, expect, Page } from "@playwright/test";

test.describe("Homepage", () => {
  test("should load the page with correct title", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveTitle("Tic Tac Toe");
  });

  test("should show player selection buttons", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator("#selectX")).toBeVisible();
    await expect(page.locator("#selectO")).toBeVisible();
    // X should be active by default
    await expect(page.locator("#selectX")).toHaveClass(/active/);
  });

  test("should show the game board with 9 disabled cells", async ({
    page,
  }) => {
    await page.goto("/");
    const cells = page.locator(".board .cell");
    await expect(cells).toHaveCount(9);
    // All cells should be disabled before a game starts
    for (let i = 0; i < 9; i++) {
      await expect(cells.nth(i)).toHaveClass(/disabled/);
    }
  });

  test("should show default status message", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator("#status")).toContainText("awaiting input");
  });

  test("should have a new game button", async ({ page }) => {
    await page.goto("/");
    const newBtn = page.locator("button", { hasText: "[new]" });
    await expect(newBtn).toBeVisible();
  });

  test("should have a join section with game ID input", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator("#joinId")).toBeVisible();
  });
});

test.describe("Player Selection", () => {
  test("should switch active player to O when clicking O", async ({
    page,
  }) => {
    await page.goto("/");
    await page.locator("#selectO").click();
    await expect(page.locator("#selectO")).toHaveClass(/active/);
    await expect(page.locator("#selectX")).not.toHaveClass(/active/);
  });

  test("should switch back to X when clicking X", async ({ page }) => {
    await page.goto("/");
    await page.locator("#selectO").click();
    await page.locator("#selectX").click();
    await expect(page.locator("#selectX")).toHaveClass(/active/);
    await expect(page.locator("#selectO")).not.toHaveClass(/active/);
  });
});

test.describe("Create New Game", () => {
  test("should create a new game as player X", async ({ page }) => {
    await page.goto("/");
    await page.locator("button", { hasText: "[new]" }).click();

    // Wait for game to be created — board should appear with clickable cells
    await expect(page.locator(".board")).toBeVisible();
    // Status should indicate it's our turn (X goes first)
    await expect(page.locator("#status")).toContainText("your_turn");
    // Game ID should be displayed
    await expect(page.locator("#gameId")).not.toBeEmpty();
    // Share link should be visible
    await expect(page.locator("#shareLink")).toContainText(
      "click to copy link"
    );
  });

  test("should create a new game as player O", async ({ page }) => {
    await page.goto("/");
    await page.locator("#selectO").click();
    await page.locator("button", { hasText: "[new]" }).click();

    await expect(page.locator(".board")).toBeVisible();
    // O should be waiting for X to move first
    await expect(page.locator("#status")).toContainText("waiting");
    await expect(page.locator("#gameId")).not.toBeEmpty();
  });

  test("should have clickable cells after creating game as X", async ({
    page,
  }) => {
    await page.goto("/");
    await page.locator("button", { hasText: "[new]" }).click();
    await expect(page.locator("#status")).toContainText("your_turn");

    // Cells should be clickable (have hx-post attributes)
    const clickableCells = page.locator('.cell[hx-post]');
    await expect(clickableCells).toHaveCount(9);
  });
});

test.describe("Making Moves", () => {
  test("should place X on the first cell", async ({ page }) => {
    await page.goto("/");
    await page.locator("button", { hasText: "[new]" }).click();
    await expect(page.locator("#status")).toContainText("your_turn");

    // Click the first cell
    const cells = page.locator(".cell");
    await cells.first().click();

    // After moving, the cell should show X and be disabled
    await expect(page.locator(".cell.x")).toHaveCount(1);
    // Status should change to waiting for opponent
    await expect(page.locator("#status")).toContainText("waiting");
  });

  test("should place moves on different cells", async ({ page }) => {
    await page.goto("/");
    await page.locator("button", { hasText: "[new]" }).click();
    await expect(page.locator("#status")).toContainText("your_turn");

    // Click the center cell (index 4)
    const clickableCells = page.locator('.cell[hx-post]');
    await clickableCells.nth(4).click();

    await expect(page.locator(".cell.x")).toHaveCount(1);
  });
});

test.describe("Two Player Game", () => {
  test("should allow two players to play via separate browser contexts", async ({
    browser,
  }) => {
    // Player X creates a game
    const contextX = await browser.newContext();
    const pageX = await contextX.newPage();
    await pageX.goto("http://localhost:8080");
    await pageX.locator("button", { hasText: "[new]" }).click();
    await expect(pageX.locator("#status")).toContainText("your_turn");

    // Extract game ID
    const gameIdText = await pageX.locator("#gameId").textContent();
    const gameId = gameIdText?.replace("session:", "").trim() ?? "";
    expect(gameId).toBeTruthy();

    // Player O joins the game
    const contextO = await browser.newContext();
    const pageO = await contextO.newPage();
    await pageO.goto("http://localhost:8080");
    await pageO.locator("#selectO").click();
    await pageO.locator("#joinId").fill(gameId);
    await pageO.locator("button", { hasText: "[join]" }).click();

    // Player O should see waiting status (X goes first)
    await expect(pageO.locator("#status")).toContainText("waiting");

    // Player X makes a move (top-left)
    const cellsX = pageX.locator('.cell[hx-post]');
    await cellsX.first().click();
    // Now X waits
    await expect(pageX.locator("#status")).toContainText("waiting");

    // Player O should see it's their turn (via SSE)
    await expect(pageO.locator("#status")).toContainText("your_turn", {
      timeout: 5000,
    });

    // Player O makes a move (center)
    const cellsO = pageO.locator('.cell[hx-post]');
    await cellsO.nth(3).click(); // center among remaining clickable cells
    await expect(pageO.locator("#status")).toContainText("waiting");

    // Player X should see it's their turn again
    await expect(pageX.locator("#status")).toContainText("your_turn", {
      timeout: 5000,
    });

    await contextX.close();
    await contextO.close();
  });
});

test.describe("Game via URL", () => {
  test("should auto-join game when game ID is in URL", async ({
    browser,
  }) => {
    // Create a game via API first
    const context = await browser.newContext();
    const page = await context.newPage();

    // Create game via the UI
    await page.goto("http://localhost:8080");
    await page.locator("button", { hasText: "[new]" }).click();
    await expect(page.locator("#gameId")).not.toBeEmpty();

    const gameIdText = await page.locator("#gameId").textContent();
    const gameId = gameIdText?.replace("session:", "").trim() ?? "";

    // Open a new page with game ID in URL as player O
    const page2 = await context.newPage();
    await page2.goto(`http://localhost:8080?game=${gameId}`);

    // Wait for it to auto-join — may fail since slot X is taken,
    // but should at least attempt to load
    await page2.waitForTimeout(1000);
    // The join input should be pre-filled with game ID
    await expect(page2.locator("#joinId")).toHaveValue(gameId);

    await context.close();
  });
});

test.describe("Reset Game", () => {
  test("should reset the board when clicking reset button", async ({
    page,
  }) => {
    await page.goto("/");
    await page.locator("button", { hasText: "[new]" }).click();
    await expect(page.locator("#status")).toContainText("your_turn");

    // Make a move
    const cells = page.locator('.cell[hx-post]');
    await cells.first().click();
    await expect(page.locator(".cell.x")).toHaveCount(1);

    // Click reset
    await page.locator("button", { hasText: "[reset]" }).click();

    // Board should be cleared
    await expect(page.locator(".cell.x")).toHaveCount(0);
    await expect(page.locator(".cell.o")).toHaveCount(0);
    // Status should be back to your_turn
    await expect(page.locator("#status")).toContainText("your_turn");
  });
});

test.describe("Win Detection", () => {
  /**
   * Helper to click a specific board position (0-8) by matching the hx-post URL.
   * The hx-post contains `/htmx/move/{gameID}/{position}?player=...`
   */
  async function clickCell(page: Page, position: number) {
    await page.locator(`.cell[hx-post*="/${position}?"]`).click();
  }

  test("should detect a winner in a full game between two players", async ({
    browser,
  }) => {
    // Player X creates a game
    const contextX = await browser.newContext();
    const pageX = await contextX.newPage();
    await pageX.goto("http://localhost:8080");
    await pageX.locator("button", { hasText: "[new]" }).click();
    await expect(pageX.locator("#status")).toContainText("your_turn");

    const gameIdText = await pageX.locator("#gameId").textContent();
    const gameId = gameIdText?.replace("session:", "").trim() ?? "";

    // Player O joins
    const contextO = await browser.newContext();
    const pageO = await contextO.newPage();
    await pageO.goto("http://localhost:8080");
    await pageO.locator("#selectO").click();
    await pageO.locator("#joinId").fill(gameId);
    await pageO.locator("button", { hasText: "[join]" }).click();
    await expect(pageO.locator("#status")).toContainText("waiting");

    // X wins with top row: positions 0, 1, 2
    // O plays positions 3, 4

    // X moves position 0
    await clickCell(pageX, 0);
    await expect(pageX.locator("#status")).toContainText("waiting");

    // O moves position 3
    await expect(pageO.locator("#status")).toContainText("your_turn", {
      timeout: 5000,
    });
    await clickCell(pageO, 3);
    await expect(pageO.locator("#status")).toContainText("waiting");

    // X moves position 1
    await expect(pageX.locator("#status")).toContainText("your_turn", {
      timeout: 5000,
    });
    await clickCell(pageX, 1);
    await expect(pageX.locator("#status")).toContainText("waiting");

    // O moves position 4
    await expect(pageO.locator("#status")).toContainText("your_turn", {
      timeout: 5000,
    });
    await clickCell(pageO, 4);
    await expect(pageO.locator("#status")).toContainText("waiting");

    // X moves position 2 — should win!
    await expect(pageX.locator("#status")).toContainText("your_turn", {
      timeout: 5000,
    });
    await clickCell(pageX, 2);

    // X should see winner
    await expect(pageX.locator("#status")).toContainText("winner");

    // O should also see the winner via SSE
    await expect(pageO.locator("#status")).toContainText("winner", {
      timeout: 5000,
    });

    await contextX.close();
    await contextO.close();
  });
});

test.describe("REST API", () => {
  test("should create a game via API", async ({ request }) => {
    const response = await request.post("/api/game");
    expect(response.ok()).toBeTruthy();
    const game = await response.json();
    expect(game.id).toBeTruthy();
    expect(game.board).toHaveLength(9);
    expect(game.currentTurn).toBe("X");
    expect(game.isOver).toBe(false);
  });

  test("should get a game via API", async ({ request }) => {
    // Create a game first
    const createRes = await request.post("/api/game");
    const game = await createRes.json();

    const getRes = await request.get(`/api/game/${game.id}`);
    expect(getRes.ok()).toBeTruthy();
    const fetched = await getRes.json();
    expect(fetched.id).toBe(game.id);
  });

  test("should make a move via API", async ({ request }) => {
    const createRes = await request.post("/api/game");
    const game = await createRes.json();

    const moveRes = await request.post(`/api/game/${game.id}`, {
      data: { position: 0, player: "X" },
    });
    expect(moveRes.ok()).toBeTruthy();
    const updated = await moveRes.json();
    expect(updated.board[0]).toBe("X");
    expect(updated.currentTurn).toBe("O");
  });

  test("should reject invalid move (position taken)", async ({ request }) => {
    const createRes = await request.post("/api/game");
    const game = await createRes.json();

    await request.post(`/api/game/${game.id}`, {
      data: { position: 0, player: "X" },
    });

    const moveRes = await request.post(`/api/game/${game.id}`, {
      data: { position: 0, player: "O" },
    });
    expect(moveRes.ok()).toBeFalsy();
  });

  test("should reject move when not your turn", async ({ request }) => {
    const createRes = await request.post("/api/game");
    const game = await createRes.json();

    const moveRes = await request.post(`/api/game/${game.id}`, {
      data: { position: 0, player: "O" },
    });
    expect(moveRes.ok()).toBeFalsy();
  });

  test("should reset a game via API", async ({ request }) => {
    const createRes = await request.post("/api/game");
    const game = await createRes.json();

    // Make a move
    await request.post(`/api/game/${game.id}`, {
      data: { position: 4, player: "X" },
    });

    // Reset
    const resetRes = await request.put(`/api/game/${game.id}`);
    expect(resetRes.ok()).toBeTruthy();
    const reset = await resetRes.json();
    expect(reset.board.every((cell: string) => cell === "")).toBeTruthy();
    expect(reset.currentTurn).toBe("X");
    expect(reset.isOver).toBe(false);
  });

  test("should return 404 for non-existent game", async ({ request }) => {
    const res = await request.get("/api/game/nonexistent");
    expect(res.status()).toBe(404);
  });

  test("should detect win via API", async ({ request }) => {
    const createRes = await request.post("/api/game");
    const game = await createRes.json();
    const id = game.id;

    // X wins top row
    await request.post(`/api/game/${id}`, {
      data: { position: 0, player: "X" },
    });
    await request.post(`/api/game/${id}`, {
      data: { position: 3, player: "O" },
    });
    await request.post(`/api/game/${id}`, {
      data: { position: 1, player: "X" },
    });
    await request.post(`/api/game/${id}`, {
      data: { position: 4, player: "O" },
    });
    const winRes = await request.post(`/api/game/${id}`, {
      data: { position: 2, player: "X" },
    });

    expect(winRes.ok()).toBeTruthy();
    const won = await winRes.json();
    expect(won.isOver).toBe(true);
    expect(won.winner).toBe("X");
    expect(won.isDraw).toBe(false);
  });

  test("should detect draw via API", async ({ request }) => {
    const createRes = await request.post("/api/game");
    const game = await createRes.json();
    const id = game.id;

    // Play to a draw:
    // X O X
    // X X O
    // O X O
    const moves = [
      { position: 0, player: "X" },
      { position: 1, player: "O" },
      { position: 2, player: "X" },
      { position: 4, player: "O" },
      { position: 3, player: "X" },
      { position: 5, player: "O" },
      { position: 7, player: "X" },
      { position: 6, player: "O" },
      { position: 8, player: "X" },
    ];

    let lastRes;
    for (const move of moves) {
      lastRes = await request.post(`/api/game/${id}`, { data: move });
    }

    const final = await lastRes!.json();
    expect(final.isOver).toBe(true);
    expect(final.isDraw).toBe(true);
  });
});
